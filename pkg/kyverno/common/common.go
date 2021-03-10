package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-git/go-billy/v5"
	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"github.com/kyverno/kyverno/pkg/utils"
	ut "github.com/kyverno/kyverno/pkg/utils"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
	yaml_v2 "sigs.k8s.io/yaml"
)

// GetPolicies - Extracting the policies from multiple YAML

type Resource struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

type Policy struct {
	Name      string     `json:"name"`
	Resources []Resource `json:"resources"`
}

type Values struct {
	Policies           []Policy            `json:"policies"`
	NamespaceSelectors []NamespaceSelector `json:"namespaceSelector"`
}

type NamespaceSelector struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

func GetPolicies(paths []string) (policies []*v1.ClusterPolicy, errors []error) {
	for _, path := range paths {
		log.Log.V(5).Info("reading policies", "path", path)

		var (
			fileDesc os.FileInfo
			err      error
		)

		isHttpPath := strings.Contains(path, "http")

		// path clean and retrieving file info can be possible if it's not an HTTP URL
		if !isHttpPath {
			path = filepath.Clean(path)
			fileDesc, err = os.Stat(path)
			if err != nil {
				err := fmt.Errorf("failed to process %v: %v", path, err.Error())
				errors = append(errors, err)
				continue
			}
		}

		// apply file from a directory is possible only if the path is not HTTP URL
		if !isHttpPath && fileDesc.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				err := fmt.Errorf("failed to process %v: %v", path, err.Error())
				errors = append(errors, err)
				continue
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				ext := filepath.Ext(file.Name())
				if ext == "" || ext == ".yaml" || ext == ".yml" {
					listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
				}
			}

			policiesFromDir, errorsFromDir := GetPolicies(listOfFiles)
			errors = append(errors, errorsFromDir...)
			policies = append(policies, policiesFromDir...)

		} else {
			var fileBytes []byte
			if isHttpPath {
				resp, err := http.Get(path)
				if err != nil {
					err := fmt.Errorf("failed to process %v: %v", path, err.Error())
					errors = append(errors, err)
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					err := fmt.Errorf("failed to process %v: %v", path, err.Error())
					errors = append(errors, err)
					continue
				}

				fileBytes, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					err := fmt.Errorf("failed to process %v: %v", path, err.Error())
					errors = append(errors, err)
					continue
				}
			} else {
				fileBytes, err = ioutil.ReadFile(path)
				if err != nil {
					err := fmt.Errorf("failed to process %v: %v", path, err.Error())
					errors = append(errors, err)
					continue
				}
			}

			policiesFromFile, errFromFile := utils.GetPolicy(fileBytes)
			if errFromFile != nil {
				err := fmt.Errorf("failed to process %s: %v", path, errFromFile.Error())
				errors = append(errors, err)
				continue
			}

			policies = append(policies, policiesFromFile...)
		}
	}

	log.Log.V(3).Info("read policies", "policies", len(policies), "errors", len(errors))
	return policies, errors
}

// PolicyHasVariables - check for variables in the policy
func PolicyHasVariables(policy v1.ClusterPolicy) [][]string {
	policyRaw, _ := json.Marshal(policy)
	matches := RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches
}

// PolicyHasNonAllowedVariables - checks for unexpected variables in the policy
func PolicyHasNonAllowedVariables(policy v1.ClusterPolicy) bool {
	policyRaw, _ := json.Marshal(policy)

	matchesAll := RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	matchesAllowed := AllowedVariables.FindAllStringSubmatch(string(policyRaw), -1)

	if len(matchesAll) > len(matchesAllowed) {
		// If rules contains Context then skip this validation
		for _, rule := range policy.Spec.Rules {
			if len(rule.Context) > 0 {
				return false
			}
		}

		return true
	}

	return false
}

// MutatePolicy - applies mutation to a policy
func MutatePolicy(policy *v1.ClusterPolicy, logger logr.Logger) (*v1.ClusterPolicy, error) {
	patches, _ := policymutation.GenerateJSONPatchesForDefaults(policy, logger)
	if len(patches) == 0 {
		return policy, nil
	}

	type jsonPatch struct {
		Path  string      `json:"path"`
		Op    string      `json:"op"`
		Value interface{} `json:"value"`
	}

	var jsonPatches []jsonPatch
	err := json.Unmarshal(patches, &jsonPatches)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to unmarshal patches for %s policy", policy.Name), err)
	}

	patch, err := jsonpatch.DecodePatch(patches)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to decode patch for %s policy", policy.Name), err)
	}

	policyBytes, _ := json.Marshal(policy)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to marshal %s policy", policy.Name), err)
	}
	modifiedPolicy, err := patch.Apply(policyBytes)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to apply %s policy", policy.Name), err)
	}

	var p v1.ClusterPolicy
	err = json.Unmarshal(modifiedPolicy, &p)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to unmarshal %s policy", policy.Name), err)
	}

	return &p, nil
}

// GetCRDs - Extracting the crds from multiple YAML
func GetCRDs(paths []string) (unstructuredCrds []*unstructured.Unstructured, err error) {
	unstructuredCrds = make([]*unstructured.Unstructured, 0)
	for _, path := range paths {
		path = filepath.Clean(path)

		fileDesc, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if fileDesc.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to parse %v", path), err)
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			policiesFromDir, err := GetCRDs(listOfFiles)
			if err != nil {
				return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to extract crds from %v", listOfFiles), err)
			}

			unstructuredCrds = append(unstructuredCrds, policiesFromDir...)
		} else {
			getCRDs, err := GetCRD(path)
			if err != nil {
				fmt.Printf("\nError: failed to extract crds from %s.  \nCause: %s\n", path, err)
				os.Exit(2)
			}
			unstructuredCrds = append(unstructuredCrds, getCRDs...)
		}
	}
	return unstructuredCrds, nil
}

// GetCRD - Extracts crds from a YAML
func GetCRD(path string) (unstructuredCrds []*unstructured.Unstructured, err error) {
	path = filepath.Clean(path)
	unstructuredCrds = make([]*unstructured.Unstructured, 0)
	yamlbytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(yamlbytes)
	reader := yaml.NewYAMLReader(bufio.NewReader(buf))

	for {
		// Read one YAML document at a time, until io.EOF is returned
		b, err := reader.Read()
		if err == io.EOF || len(b) == 0 {
			break
		} else if err != nil {
			fmt.Printf("\nError: unable to read crd from %s. Cause: %s\n", path, err)
			os.Exit(2)
		}
		var u unstructured.Unstructured
		err = yaml_v2.Unmarshal(b, &u)
		if err != nil {
			return nil, err
		}
		unstructuredCrds = append(unstructuredCrds, &u)
	}

	return unstructuredCrds, nil
}

// IsInputFromPipe - check if input is passed using pipe
func IsInputFromPipe() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo.Mode()&os.ModeCharDevice == 0
}

// RemoveDuplicateVariables - remove duplicate variables
func RemoveDuplicateVariables(matches [][]string) string {
	var variableStr string
	for _, m := range matches {
		for _, v := range m {
			foundVariable := strings.Contains(variableStr, v)
			if !foundVariable {
				variableStr = variableStr + " " + v
			}
		}
	}
	return variableStr
}

// GetVariable - get the variables from console/file
func GetVariable(variablesString, valuesFile string, fs billy.Filesystem, isGit bool, policyresoucePath string) (map[string]string, map[string]map[string]Resource, map[string]map[string]string, error) {
	valuesMap := make(map[string]map[string]Resource)
	namespaceSelectorMap := make(map[string]map[string]string)
	variables := make(map[string]string)
	var yamlFile []byte
	var err error
	if variablesString != "" {
		kvpairs := strings.Split(strings.Trim(variablesString, " "), ",")
		for _, kvpair := range kvpairs {
			kvs := strings.Split(strings.Trim(kvpair, " "), "=")
			variables[strings.Trim(kvs[0], " ")] = strings.Trim(kvs[1], " ")
		}
	}
	if valuesFile != "" {
		if isGit {
			filep, err := fs.Open(filepath.Join(policyresoucePath, valuesFile))
			if err != nil {
				fmt.Printf("Unable to open variable file: %s. error: %s", valuesFile, err)
			}
			yamlFile, err = ioutil.ReadAll(filep)
		} else {
			yamlFile, err = ioutil.ReadFile(valuesFile)
		}

		if err != nil {
			return variables, valuesMap, namespaceSelectorMap, sanitizederror.NewWithError("unable to read yaml", err)
		}

		valuesBytes, err := yaml.ToJSON(yamlFile)
		if err != nil {
			return variables, valuesMap, namespaceSelectorMap, sanitizederror.NewWithError("failed to convert json", err)
		}

		values := &Values{}
		if err := json.Unmarshal(valuesBytes, values); err != nil {
			return variables, valuesMap, namespaceSelectorMap, sanitizederror.NewWithError("failed to decode yaml", err)
		}

		for _, p := range values.Policies {
			pmap := make(map[string]Resource)
			for _, r := range p.Resources {
				pmap[r.Name] = r
			}
			valuesMap[p.Name] = pmap
		}

		for _, n := range values.NamespaceSelectors {
			namespaceSelectorMap[n.Name] = n.Labels
		}
	}

	return variables, valuesMap, namespaceSelectorMap, nil
}

// MutatePolices - function to apply mutation on policies
func MutatePolices(policies []*v1.ClusterPolicy) ([]*v1.ClusterPolicy, error) {
	newPolicies := make([]*v1.ClusterPolicy, 0)
	logger := log.Log.WithName("apply")

	for _, policy := range policies {
		p, err := MutatePolicy(policy, logger)
		if err != nil {
			if !sanitizederror.IsErrorSanitized(err) {
				return nil, sanitizederror.NewWithError("failed to mutate policy.", err)
			}
			return nil, err
		}
		newPolicies = append(newPolicies, p)
	}
	return newPolicies, nil
}

// ApplyPolicyOnResource - function to apply policy on resource
func ApplyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured,
	mutateLogPath string, mutateLogPathIsDir bool, variables map[string]string, policyReport bool, namespaceSelectorMap map[string]map[string]string) ([]*response.EngineResponse, *response.EngineResponse, bool, bool, error) {

	responseError := false
	rcError := false
	engineResponses := make([]*response.EngineResponse, 0)
	namespaceLabels := make(map[string]string)
	resourceNamespace := resource.GetNamespace()
	namespaceLabels = namespaceSelectorMap[resource.GetNamespace()]

	if resourceNamespace != "default" && len(namespaceLabels) < 1 {
		return engineResponses, &response.EngineResponse{}, responseError, rcError, sanitizederror.NewWithError(fmt.Sprintf("failed to get namesapce labels for resource %s. use --values-file flag to pass the namespace labels", resource.GetName()), nil)
	}
	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	log.Log.V(3).Info("applying policy on resource", "policy", policy.Name, "resource", resPath)

	ctx := context.NewContext()
	for key, value := range variables {
		var subString string
		splitBySlash := strings.Split(key, "\"")
		if len(splitBySlash) > 1 {
			subString = splitBySlash[1]
		}

		startString := ""
		endString := ""
		lenOfVariableString := 0
		addedSlashString := false
		for _, k := range strings.Split(splitBySlash[0], ".") {
			if k != "" {
				startString += fmt.Sprintf(`{"%s":`, k)
				endString += `}`
				lenOfVariableString = lenOfVariableString + len(k) + 1
				if lenOfVariableString >= len(splitBySlash[0]) && len(splitBySlash) > 1 && addedSlashString == false {
					startString += fmt.Sprintf(`{"%s":`, subString)
					endString += `}`
					addedSlashString = true
				}
			}
		}

		midString := fmt.Sprintf(`"%s"`, value)
		finalString := startString + midString + endString
		var jsonData = []byte(finalString)
		ctx.AddJSON(jsonData)
	}

	mutateResponse := engine.Mutate(&engine.PolicyContext{Policy: *policy, NewResource: *resource, JSONContext: ctx, NamespaceLabels: namespaceLabels})
	engineResponses = append(engineResponses, mutateResponse)

	if !mutateResponse.IsSuccessful() {
		fmt.Printf("Failed to apply mutate policy %s -> resource %s", policy.Name, resPath)
		for i, r := range mutateResponse.PolicyResponse.Rules {
			fmt.Printf("\n%d. %s", i+1, r.Message)
		}
		responseError = true
	} else {
		if len(mutateResponse.PolicyResponse.Rules) > 0 {
			yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
			if err != nil {
				rcError = true
			}

			if mutateLogPath == "" {
				mutatedResource := string(yamlEncodedResource)
				if len(strings.TrimSpace(mutatedResource)) > 0 {
					fmt.Printf("\nmutate policy %s applied to %s:", policy.Name, resPath)
					fmt.Printf("\n" + mutatedResource)
					fmt.Printf("\n")
				}
			} else {
				err := PrintMutatedOutput(mutateLogPath, mutateLogPathIsDir, string(yamlEncodedResource), resource.GetName()+"-mutated")
				if err != nil {
					return engineResponses, &response.EngineResponse{}, responseError, rcError, sanitizederror.NewWithError("failed to print mutated result", err)
				}
				fmt.Printf("\n\nMutation:\nMutation has been applied successfully. Check the files.")
			}

		}
	}

	if resource.GetKind() == "Pod" && len(resource.GetOwnerReferences()) > 0 {
		if policy.HasAutoGenAnnotation() {
			if _, ok := policy.GetAnnotations()[engine.PodControllersAnnotation]; ok {
				delete(policy.Annotations, engine.PodControllersAnnotation)
			}
		}
	}

	policyCtx := &engine.PolicyContext{Policy: *policy, NewResource: mutateResponse.PatchedResource, JSONContext: ctx, NamespaceLabels: namespaceLabels}
	validateResponse := engine.Validate(policyCtx)
	if !policyReport {
		if !validateResponse.IsSuccessful() {
			fmt.Printf("\npolicy %s -> resource %s failed: \n", policy.Name, resPath)
			for i, r := range validateResponse.PolicyResponse.Rules {
				if !r.Success {
					fmt.Printf("%d. %s: %s \n", i+1, r.Name, r.Message)
				}
			}

			responseError = true
		}
	}

	var policyHasGenerate bool
	for _, rule := range policy.Spec.Rules {
		if rule.HasGenerate() {
			policyHasGenerate = true
		}
	}

	if policyHasGenerate {
		policyContext := &engine.PolicyContext{
			NewResource:      *resource,
			Policy:           *policy,
			ExcludeGroupRole: []string{},
			ExcludeResourceFunc: func(s1, s2, s3 string) bool {
				return false
			},
			JSONContext:     context.NewContext(),
			NamespaceLabels: namespaceLabels,
		}
		generateResponse := engine.Generate(policyContext)
		engineResponses = append(engineResponses, generateResponse)
		if len(generateResponse.PolicyResponse.Rules) > 0 {
			log.Log.V(3).Info("generate resource is valid", "policy", policy.Name, "resource", resPath)
		} else {
			fmt.Printf("generate policy %s resource %s is invalid \n", policy.Name, resPath)
			for i, r := range generateResponse.PolicyResponse.Rules {
				fmt.Printf("%d. %s \b", i+1, r.Message)
			}

			responseError = true
		}
	}

	return engineResponses, validateResponse, responseError, rcError, nil
}

// PrintMutatedOutput - function to print output in provided file or directory
func PrintMutatedOutput(mutateLogPath string, mutateLogPathIsDir bool, yaml string, fileName string) error {
	var f *os.File
	var err error
	yaml = yaml + ("\n---\n\n")

	if !mutateLogPathIsDir {
		f, err = os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		f, err = os.OpenFile(mutateLogPath+"/"+fileName+".yaml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	}

	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(yaml)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

// GetPoliciesFromPaths - get policies according to the resource path
func GetPoliciesFromPaths(fs billy.Filesystem, dirPath []string, isGit bool, policyresoucePath string) (policies []*v1.ClusterPolicy, err error) {
	var errors []error
	if isGit {
		for _, pp := range dirPath {
			filep, err := fs.Open(filepath.Join(policyresoucePath, pp))
			if err != nil {
				fmt.Printf("Error: file not available with path %s: %v", filep.Name(), err.Error())
				continue
			}
			bytes, err := ioutil.ReadAll(filep)
			if err != nil {
				fmt.Printf("Error: failed to read file %s: %v", filep.Name(), err.Error())
				continue
			}
			policyBytes, err := yaml.ToJSON(bytes)
			if err != nil {
				fmt.Printf("failed to convert to JSON: %v", err)
				continue
			}
			policiesFromFile, errFromFile := ut.GetPolicy(policyBytes)
			if errFromFile != nil {
				err := fmt.Errorf("failed to process : %v", errFromFile.Error())
				errors = append(errors, err)
				continue
			}
			policies = append(policies, policiesFromFile...)
		}
	} else {
		if len(dirPath) > 0 && dirPath[0] == "-" {
			if IsInputFromPipe() {
				policyStr := ""
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					policyStr = policyStr + scanner.Text() + "\n"
				}
				yamlBytes := []byte(policyStr)
				policies, err = ut.GetPolicy(yamlBytes)
				if err != nil {
					return nil, sanitizederror.NewWithError("failed to extract the resources", err)
				}
			}
		} else {
			var errors []error
			policies, errors = GetPolicies(dirPath)
			if len(policies) == 0 {
				if len(errors) > 0 {
					return nil, sanitizederror.NewWithErrors("failed to read file", errors)
				}
				return nil, sanitizederror.New(fmt.Sprintf("no file found in paths %v", dirPath))
			}
			if len(errors) > 0 && log.Log.V(1).Enabled() {
				fmt.Printf("ignoring errors: \n")
				for _, e := range errors {
					fmt.Printf("    %v \n", e.Error())
				}
			}
		}
	}
	return
}

// GetResourceAccordingToResourcePath - get resources according to the resource path
func GetResourceAccordingToResourcePath(fs billy.Filesystem, resourcePaths []string,
	cluster bool, policies []*v1.ClusterPolicy, dClient *client.Client, namespace string, policyReport bool, isGit bool, policyresoucePath string) (resources []*unstructured.Unstructured, err error) {
	if isGit {
		resources, err = GetResourcesWithTest(fs, policies, resourcePaths, isGit, policyresoucePath)
		if err != nil {
			return nil, sanitizederror.NewWithError("failed to extract the resources", err)
		}
	} else {
		if len(resourcePaths) > 0 && resourcePaths[0] == "-" {
			if IsInputFromPipe() {
				resourceStr := ""
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					resourceStr = resourceStr + scanner.Text() + "\n"
				}

				yamlBytes := []byte(resourceStr)
				resources, err = GetResource(yamlBytes)
				if err != nil {
					return nil, sanitizederror.NewWithError("failed to extract the resources", err)
				}
			}
		} else if (len(resourcePaths) > 0 && resourcePaths[0] != "-") || len(resourcePaths) < 0 || cluster {
			resources, err = GetResources(policies, resourcePaths, dClient, cluster, namespace, policyReport)
			if err != nil {
				return resources, err
			}
		}

	}
	return resources, err
}
