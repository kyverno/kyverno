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
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-git/go-billy/v5"
	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	pkgcommon "github.com/kyverno/kyverno/pkg/common"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/kyverno/store"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/utils"
	ut "github.com/kyverno/kyverno/pkg/utils"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

type ResultCounts struct {
	Pass  int
	Fail  int
	Warn  int
	Error int
	Skip  int
}
type Policy struct {
	Name      string     `json:"name"`
	Resources []Resource `json:"resources"`
	Rules     []Rule     `json:"rules"`
}

type Rule struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

type Values struct {
	Policies           []Policy            `json:"policies"`
	GlobalValues       map[string]string   `json:"globalValues"`
	NamespaceSelectors []NamespaceSelector `json:"namespaceSelector"`
}

type Resource struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

type NamespaceSelector struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

// GetPolicies - Extracting the policies from multiple YAML
func GetPolicies(paths []string) (policies []*v1.ClusterPolicy, errors []error) {
	for _, path := range paths {
		log.Log.V(5).Info("reading policies", "path", path)

		var (
			fileDesc os.FileInfo
			err      error
		)

		isHTTPPath := IsHTTPRegex.MatchString(path)

		// path clean and retrieving file info can be possible if it's not an HTTP URL
		if !isHTTPPath {
			path = filepath.Clean(path)
			fileDesc, err = os.Stat(path)
			if err != nil {
				err := fmt.Errorf("failed to process %v: %v", path, err.Error())
				errors = append(errors, err)
				continue
			}
		}

		// apply file from a directory is possible only if the path is not HTTP URL
		if !isHTTPPath && fileDesc.IsDir() {
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
			if isHTTPPath {
				// We accept here that a random URL might be called based on user provided input.
				resp, err := http.Get(path) // #nosec
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
				path = filepath.Clean(path)
				// We accept the risk of including a user provided file here.
				fileBytes, err = ioutil.ReadFile(path) // #nosec G304
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

// for now forbidden sections are match, exclude and
func ruleForbiddenSectionsHaveVariables(rule *v1.Rule) error {
	var err error

	err = JSONPatchPathHasVariables(rule.Mutation.PatchesJSON6902)
	if err != nil {
		return fmt.Errorf("Rule \"%s\" should not have variables in patchesJSON6902 path section", rule.Name)
	}

	err = objectHasVariables(rule.ExcludeResources)
	if err != nil {
		return fmt.Errorf("Rule \"%s\" should not have variables in exclude section", rule.Name)
	}

	err = objectHasVariables(rule.MatchResources)
	if err != nil {
		return fmt.Errorf("Rule \"%s\" should not have variables in match section", rule.Name)
	}

	return nil
}

func JSONPatchPathHasVariables(patch string) error {
	jsonPatch, err := yaml.ToJSON([]byte(patch))
	if err != nil {
		return err
	}

	decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return err
	}

	for _, operation := range decodedPatch {
		path, err := operation.Path()
		if err != nil {
			return err
		}

		vars := variables.RegexVariables.FindAllString(path, -1)
		if len(vars) > 0 {
			return fmt.Errorf("Operation \"%s\" has forbidden variables", operation.Kind())
		}
	}

	return nil
}

func objectHasVariables(object interface{}) error {
	var err error
	objectJSON, err := json.Marshal(object)
	if err != nil {
		return err
	}

	if len(RegexVariables.FindAllStringSubmatch(string(objectJSON), -1)) > 0 {
		return fmt.Errorf("Object has forbidden variables")
	}

	return nil
}

// PolicyHasNonAllowedVariables - checks for unexpected variables in the policy
func PolicyHasNonAllowedVariables(policy v1.ClusterPolicy) error {
	for _, r := range policy.Spec.Rules {
		rule := r.DeepCopy()

		// do not validate attestation variables as they are based on external data
		for _, vi := range rule.VerifyImages {
			vi.Attestations = nil
		}

		var err error
		ruleJSON, err := json.Marshal(rule)
		if err != nil {
			return err
		}

		err = ruleForbiddenSectionsHaveVariables(rule)
		if err != nil {
			return err
		}

		matchesAll := RegexVariables.FindAllStringSubmatch(string(ruleJSON), -1)
		matchesAllowed := AllowedVariables.FindAllStringSubmatch(string(ruleJSON), -1)
		if (len(matchesAll) > len(matchesAllowed)) && len(rule.Context) == 0 {
			allowed := "{{request.*}}, {{element.*}}, {{serviceAccountName}}, {{serviceAccountNamespace}}, {{@}}, {{images.*}} and context variables"
			return fmt.Errorf("rule \"%s\" has forbidden variables. Allowed variables are: %s", rule.Name, allowed)
		}
	}

	return nil
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
	// We accept the risk of including a user provided file here.
	yamlbytes, err := ioutil.ReadFile(path) // #nosec G304
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
		err = k8syaml.Unmarshal(b, &u)
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

// RemoveDuplicateAndObjectVariables - remove duplicate variables
func RemoveDuplicateAndObjectVariables(matches [][]string) string {
	var variableStr string
	for _, m := range matches {
		for _, v := range m {
			foundVariable := strings.Contains(variableStr, v)
			if !foundVariable {
				if !strings.Contains(v, "request.object") && !strings.Contains(v, "element") {
					variableStr = variableStr + " " + v
				}
			}
		}
	}
	return variableStr
}

func GetVariable(variablesString, valuesFile string, fs billy.Filesystem, isGit bool, policyResourcePath string) (map[string]string, map[string]string, map[string]map[string]Resource, map[string]map[string]string, error) {
	valuesMapResource := make(map[string]map[string]Resource)
	valuesMapRule := make(map[string]map[string]Rule)
	namespaceSelectorMap := make(map[string]map[string]string)
	variables := make(map[string]string)
	globalValMap := make(map[string]string)
	reqObjVars := ""

	var yamlFile []byte
	var err error
	if variablesString != "" {
		kvpairs := strings.Split(strings.Trim(variablesString, " "), ",")
		for _, kvpair := range kvpairs {
			kvs := strings.Split(strings.Trim(kvpair, " "), "=")
			if strings.Contains(kvs[0], "request.object") {
				if !strings.Contains(reqObjVars, kvs[0]) {
					reqObjVars = reqObjVars + "," + kvs[0]
				}
				continue
			}

			variables[strings.Trim(kvs[0], " ")] = strings.Trim(kvs[1], " ")
		}
	}

	if valuesFile != "" {
		if isGit {
			filep, err := fs.Open(filepath.Join(policyResourcePath, valuesFile))
			if err != nil {
				fmt.Printf("Unable to open variable file: %s. error: %s", valuesFile, err)
			}
			yamlFile, err = ioutil.ReadAll(filep)
			if err != nil {
				fmt.Printf("Unable to read variable files: %s. error: %s \n", filep, err)
			}
		} else {
			// We accept the risk of including a user provided file here.
			yamlFile, err = ioutil.ReadFile(filepath.Join(policyResourcePath, valuesFile)) // #nosec G304
			if err != nil {
				fmt.Printf("\n Unable to open variable file: %s. error: %s \n", valuesFile, err)
			}
		}

		if err != nil {
			return variables, globalValMap, valuesMapResource, namespaceSelectorMap, sanitizederror.NewWithError("unable to read yaml", err)
		}

		valuesBytes, err := yaml.ToJSON(yamlFile)
		if err != nil {
			return variables, globalValMap, valuesMapResource, namespaceSelectorMap, sanitizederror.NewWithError("failed to convert json", err)
		}

		values := &Values{}
		if err := json.Unmarshal(valuesBytes, values); err != nil {
			return variables, globalValMap, valuesMapResource, namespaceSelectorMap, sanitizederror.NewWithError("failed to decode yaml", err)
		}

		globalValMap = values.GlobalValues

		for _, p := range values.Policies {
			resourceMap := make(map[string]Resource)
			for _, r := range p.Resources {
				for variableInFile := range r.Values {
					if strings.Contains(variableInFile, "request.object") {
						if !strings.Contains(reqObjVars, variableInFile) {
							reqObjVars = reqObjVars + "," + variableInFile
						}
						delete(r.Values, variableInFile)
						continue
					}
				}
				resourceMap[r.Name] = r
			}
			valuesMapResource[p.Name] = resourceMap

			if p.Rules != nil {
				ruleMap := make(map[string]Rule)
				for _, r := range p.Rules {
					ruleMap[r.Name] = r
				}
				valuesMapRule[p.Name] = ruleMap
			}
		}

		for _, n := range values.NamespaceSelectors {
			namespaceSelectorMap[n.Name] = n.Labels
		}
	}

	if reqObjVars != "" {
		fmt.Printf(("\nNOTICE: request.object.* variables are automatically parsed from the supplied resource. Ignoring value of variables `%v`.\n"), reqObjVars)
	}

	storePolices := make([]store.Policy, 0)
	for policyName, ruleMap := range valuesMapRule {
		storeRules := make([]store.Rule, 0)
		for _, rule := range ruleMap {
			storeRules = append(storeRules, store.Rule{
				Name:   rule.Name,
				Values: rule.Values,
			})
		}
		storePolices = append(storePolices, store.Policy{
			Name:  policyName,
			Rules: storeRules,
		})
	}

	store.SetContext(store.Context{
		Policies: storePolices,
	})

	return variables, globalValMap, valuesMapResource, namespaceSelectorMap, nil
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
	mutateLogPath string, mutateLogPathIsDir bool, variables map[string]string, policyReport bool, namespaceSelectorMap map[string]map[string]string, stdin bool, rc *ResultCounts, printPatchResource bool) ([]*response.EngineResponse, policyreport.Info, error) {

	var engineResponses []*response.EngineResponse
	namespaceLabels := make(map[string]string)
	operationIsDelete := false

	if variables["request.operation"] == "DELETE" {
		operationIsDelete = true
	}

	policyWithNamespaceSelector := false
	for _, p := range policy.Spec.Rules {
		if p.MatchResources.ResourceDescription.NamespaceSelector != nil ||
			p.ExcludeResources.ResourceDescription.NamespaceSelector != nil {
			policyWithNamespaceSelector = true
			break
		}
	}

	if policyWithNamespaceSelector {
		resourceNamespace := resource.GetNamespace()
		namespaceLabels = namespaceSelectorMap[resource.GetNamespace()]
		if resourceNamespace != "default" && len(namespaceLabels) < 1 {
			return engineResponses, policyreport.Info{}, sanitizederror.NewWithError(fmt.Sprintf("failed to get namesapce labels for resource %s. use --values-file flag to pass the namespace labels", resource.GetName()), nil)
		}
	}

	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	log.Log.V(3).Info("applying policy on resource", "policy", policy.Name, "resource", resPath)

	ctx := context.NewContext()
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		log.Log.Error(err, "failed to marshal resource")
	}

	if operationIsDelete {
		err = ctx.AddResourceInOldObject(resourceRaw)
	} else {
		err = ctx.AddResource(resourceRaw)
	}
	if err != nil {
		log.Log.Error(err, "failed to load resource in context")
	}

	for key, value := range variables {
		jsonData := pkgcommon.VariableToJSON(key, value)
		err = ctx.AddJSON(jsonData)
		if err != nil {
			log.Log.Error(err, "failed to add variable to context")
		}
	}

	mutateResponse := engine.Mutate(&engine.PolicyContext{Policy: *policy, NewResource: *resource, JSONContext: ctx, NamespaceLabels: namespaceLabels})
	if mutateResponse != nil {
		engineResponses = append(engineResponses, mutateResponse)
	}

	err = processMutateEngineResponse(policy, mutateResponse, resPath, rc, mutateLogPath, stdin, mutateLogPathIsDir, resource.GetName(), printPatchResource)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return engineResponses, policyreport.Info{}, sanitizederror.NewWithError("failed to print mutated result", err)
		}
	}

	if resource.GetKind() == "Pod" && len(resource.GetOwnerReferences()) > 0 {
		if policy.HasAutoGenAnnotation() {
			if _, ok := policy.GetAnnotations()[engine.PodControllersAnnotation]; ok {
				delete(policy.Annotations, engine.PodControllersAnnotation)
			}
		}
	}

	var policyHasValidate bool
	for _, rule := range policy.Spec.Rules {
		if rule.HasValidate() {
			policyHasValidate = true
		}
	}

	var info policyreport.Info
	var validateResponse *response.EngineResponse
	if policyHasValidate {
		policyCtx := &engine.PolicyContext{Policy: *policy, NewResource: mutateResponse.PatchedResource, JSONContext: ctx, NamespaceLabels: namespaceLabels}
		validateResponse = engine.Validate(policyCtx)
		info = ProcessValidateEngineResponse(policy, validateResponse, resPath, rc, policyReport)
	}
	if validateResponse != nil {
		engineResponses = append(engineResponses, validateResponse)
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
		if generateResponse != nil {
			engineResponses = append(engineResponses, generateResponse)
		}
		processGenerateEngineResponse(policy, generateResponse, resPath, rc)
	}

	return engineResponses, info, nil
}

// PrintMutatedOutput - function to print output in provided file or directory
func PrintMutatedOutput(mutateLogPath string, mutateLogPathIsDir bool, yaml string, fileName string) error {
	var f *os.File
	var err error
	yaml = yaml + ("\n---\n\n")

	mutateLogPath = filepath.Clean(mutateLogPath)
	if !mutateLogPathIsDir {
		// truncation for the case when mutateLogPath is a file (not a directory) is handled under pkg/kyverno/apply/test_command.go
		f, err = os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0600) // #nosec G304
	} else {
		f, err = os.OpenFile(mutateLogPath+"/"+fileName+".yaml", os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304
	}

	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(yaml)); err != nil {
		closeErr := f.Close()
		if closeErr != nil {
			log.Log.Error(closeErr, "failed to close file")
		}
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

// GetPoliciesFromPaths - get policies according to the resource path
func GetPoliciesFromPaths(fs billy.Filesystem, dirPath []string, isGit bool, policyResourcePath string) (policies []*v1.ClusterPolicy, err error) {
	var errors []error
	if isGit {
		for _, pp := range dirPath {
			filep, err := fs.Open(filepath.Join(policyResourcePath, pp))
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
	cluster bool, policies []*v1.ClusterPolicy, dClient *client.Client, namespace string, policyReport bool, isGit bool, policyResourcePath string) (resources []*unstructured.Unstructured, err error) {
	if isGit {
		resources, err = GetResourcesWithTest(fs, policies, resourcePaths, isGit, policyResourcePath)
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

func ProcessValidateEngineResponse(policy *v1.ClusterPolicy, validateResponse *response.EngineResponse, resPath string, rc *ResultCounts, policyReport bool) policyreport.Info {
	var violatedRules []v1.ViolatedRule
	printCount := 0
	for _, policyRule := range policy.Spec.Rules {
		ruleFoundInEngineResponse := false

		for i, valResponseRule := range validateResponse.PolicyResponse.Rules {
			if policyRule.Name == valResponseRule.Name {
				ruleFoundInEngineResponse = true
				vrule := v1.ViolatedRule{
					Name:    valResponseRule.Name,
					Type:    valResponseRule.Type,
					Message: valResponseRule.Message,
				}

				switch valResponseRule.Status {
				case response.RuleStatusPass:
					rc.Pass++
					vrule.Status = report.StatusPass

				case response.RuleStatusFail:
					rc.Fail++
					vrule.Status = report.StatusFail
					if !policyReport {
						if printCount < 1 {
							fmt.Printf("\npolicy %s -> resource %s failed: \n", policy.Name, resPath)
							printCount++
						}

						fmt.Printf("%d. %s: %s \n", i+1, valResponseRule.Name, valResponseRule.Message)
					}

				case response.RuleStatusError:
					rc.Error++
					vrule.Status = report.StatusError

				case response.RuleStatusWarn:
					rc.Warn++
					vrule.Status = report.StatusWarn

				case response.RuleStatusSkip:
					rc.Skip++
					vrule.Status = report.StatusSkip
				}

				violatedRules = append(violatedRules, vrule)
				continue
			}
		}

		if !ruleFoundInEngineResponse {
			rc.Skip++
			vruleSkip := v1.ViolatedRule{
				Name:    policyRule.Name,
				Type:    "Validation",
				Message: policyRule.Validation.Message,
				Status:  report.StatusSkip,
			}
			violatedRules = append(violatedRules, vruleSkip)
		}

	}
	return buildPVInfo(validateResponse, violatedRules)
}

func buildPVInfo(er *response.EngineResponse, violatedRules []v1.ViolatedRule) policyreport.Info {
	info := policyreport.Info{
		PolicyName: er.PolicyResponse.Policy.Name,
		Namespace:  er.PatchedResource.GetNamespace(),
		Results: []policyreport.EngineResponseResult{
			{
				Resource: er.GetResourceSpec(),
				Rules:    violatedRules,
			},
		},
	}
	return info
}

func processGenerateEngineResponse(policy *v1.ClusterPolicy, generateResponse *response.EngineResponse, resPath string, rc *ResultCounts) {
	printCount := 0
	for _, policyRule := range policy.Spec.Rules {
		ruleFoundInEngineResponse := false
		for i, genResponseRule := range generateResponse.PolicyResponse.Rules {
			if policyRule.Name == genResponseRule.Name {
				ruleFoundInEngineResponse = true
				if genResponseRule.Status == response.RuleStatusPass {
					rc.Pass++
				} else {
					if printCount < 1 {
						fmt.Println("\ngenerate resource is not valid", "policy", policy.Name, "resource", resPath)
						printCount++
					}
					fmt.Printf("%d. %s - %s\n", i+1, genResponseRule.Name, genResponseRule.Message)
					rc.Fail++
				}
				continue
			}
		}
		if !ruleFoundInEngineResponse {
			rc.Skip++
		}
	}
}

func SetInStoreContext(mutatedPolicies []*v1.ClusterPolicy, variables map[string]string) map[string]string {
	storePolices := make([]store.Policy, 0)
	for _, policy := range mutatedPolicies {
		storeRules := make([]store.Rule, 0)
		for _, rule := range policy.Spec.Rules {
			contextVal := make(map[string]string)
			if len(rule.Context) != 0 {
				for _, contextVar := range rule.Context {
					for k, v := range variables {
						if strings.HasPrefix(k, contextVar.Name) {
							contextVal[k] = v
							delete(variables, k)
						}
					}
				}
				storeRules = append(storeRules, store.Rule{
					Name:   rule.Name,
					Values: contextVal,
				})
			}
		}
		storePolices = append(storePolices, store.Policy{
			Name:  policy.Name,
			Rules: storeRules,
		})
	}

	store.SetContext(store.Context{
		Policies: storePolices,
	})

	return variables
}

func processMutateEngineResponse(policy *v1.ClusterPolicy, mutateResponse *response.EngineResponse, resPath string, rc *ResultCounts, mutateLogPath string, stdin bool, mutateLogPathIsDir bool, resourceName string, printPatchResource bool) error {
	var policyHasMutate bool
	for _, rule := range policy.Spec.Rules {
		if rule.HasMutate() {
			policyHasMutate = true
		}
	}
	if !policyHasMutate {
		return nil
	}

	printCount := 0
	printMutatedRes := false
	for _, policyRule := range policy.Spec.Rules {
		ruleFoundInEngineResponse := false
		for i, mutateResponseRule := range mutateResponse.PolicyResponse.Rules {
			if policyRule.Name == mutateResponseRule.Name {
				ruleFoundInEngineResponse = true
				if mutateResponseRule.Status == response.RuleStatusPass {
					rc.Pass++
					printMutatedRes = true
				} else if mutateResponseRule.Status == response.RuleStatusSkip {
					fmt.Printf("\nskipped mutate policy %s -> resource %s", policy.Name, resPath)
					rc.Skip++
				} else if mutateResponseRule.Status == response.RuleStatusError {
					fmt.Printf("\nerror while applying mutate policy %s -> resource %s\nerror: %s", policy.Name, resPath, mutateResponseRule.Message)
					rc.Error++
				} else {
					if printCount < 1 {
						fmt.Printf("\nfailed to apply mutate policy %s -> resource %s", policy.Name, resPath)
						printCount++
					}
					fmt.Printf("%d. %s - %s \n", i+1, mutateResponseRule.Name, mutateResponseRule.Message)
					rc.Fail++
				}
				continue
			}
		}
		if !ruleFoundInEngineResponse {
			rc.Skip++
		}
	}

	if printMutatedRes && printPatchResource {
		yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
		if err != nil {
			return sanitizederror.NewWithError("failed to marshal", err)
		}

		if mutateLogPath == "" {
			mutatedResource := string(yamlEncodedResource) + string("\n---")
			if len(strings.TrimSpace(mutatedResource)) > 0 {
				if !stdin {
					fmt.Printf("\nmutate policy %s applied to %s:", policy.Name, resPath)
				}
				fmt.Printf("\n" + mutatedResource + "\n")
			}
		} else {
			err := PrintMutatedOutput(mutateLogPath, mutateLogPathIsDir, string(yamlEncodedResource), resourceName+"-mutated")
			if err != nil {
				return sanitizederror.NewWithError("failed to print mutated result", err)
			}
			fmt.Printf("\n\nMutation:\nMutation has been applied successfully. Check the files.")
		}
	}

	return nil
}

func PrintMutatedPolicy(mutatedPolicies []*v1.ClusterPolicy) error {
	for _, policy := range mutatedPolicies {
		p, err := json.Marshal(policy)
		if err != nil {
			return sanitizederror.NewWithError("failed to marsal mutated policy", err)
		}
		log.Log.V(5).Info("mutated Policy:", string(p))
	}
	return nil
}

func CheckVariableForPolicy(valuesMap map[string]map[string]Resource, globalValMap map[string]string, policyName string, resourceName string, resourceKind string, variables map[string]string, kindOnwhichPolicyIsApplied map[string]struct{}, variable string) (map[string]string, error) {
	// get values from file for this policy resource combination
	thisPolicyResourceValues := make(map[string]string)
	if len(valuesMap[policyName]) != 0 && !reflect.DeepEqual(valuesMap[policyName][resourceName], Resource{}) {
		thisPolicyResourceValues = valuesMap[policyName][resourceName].Values
	}

	for k, v := range variables {
		thisPolicyResourceValues[k] = v
	}

	if thisPolicyResourceValues == nil && len(globalValMap) > 0 {
		thisPolicyResourceValues = make(map[string]string)
	}

	for k, v := range globalValMap {
		if _, ok := thisPolicyResourceValues[k]; !ok {
			thisPolicyResourceValues[k] = v
		}
	}

	// skipping the variable check for non matching kind
	if _, ok := kindOnwhichPolicyIsApplied[resourceKind]; ok {
		if len(variable) > 0 && len(thisPolicyResourceValues) == 0 && len(store.GetContext().Policies) == 0 {
			return thisPolicyResourceValues, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policyName, resourceName), nil)
		}
	}
	return thisPolicyResourceValues, nil
}

func GetKindsFromPolicy(policy *v1.ClusterPolicy) map[string]struct{} {
	var kindOnwhichPolicyIsApplied = make(map[string]struct{})
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
			kindOnwhichPolicyIsApplied[kind] = struct{}{}
		}
		for _, kind := range rule.ExcludeResources.ResourceDescription.Kinds {
			kindOnwhichPolicyIsApplied[kind] = struct{}{}
		}
	}
	return kindOnwhichPolicyIsApplied
}

//GetPatchedResourceFromPath - get patchedResource from given path
func GetPatchedResourceFromPath(fs billy.Filesystem, path string, isGit bool, policyResourcePath string) (unstructured.Unstructured, error) {
	var patchedResourceBytes []byte
	var patchedResource unstructured.Unstructured
	var err error

	if isGit {
		if len(path) > 0 {
			filep, err := fs.Open(filepath.Join(policyResourcePath, path))
			if err != nil {
				fmt.Printf("Unable to open patchedResource file: %s. \nerror: %s", path, err)
			}
			patchedResourceBytes, err = ioutil.ReadAll(filep)
		}
	} else {
		patchedResourceBytes, err = getFileBytes(path)
	}

	if err != nil {
		fmt.Printf("\n----------------------------------------------------------------------\nfailed to load patchedResource: %s. \nerror: %s\n----------------------------------------------------------------------\n", path, err)
		return patchedResource, err
	}

	patchedResource, err = GetPatchedResource(patchedResourceBytes)
	if err != nil {
		return patchedResource, err
	}

	return patchedResource, nil
}
