package common

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	engineContext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/registryclient"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	yamlv2 "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	Name          string                   `json:"name"`
	Values        map[string]interface{}   `json:"values"`
	ForeachValues map[string][]interface{} `json:"foreachValues"`
}

type Values struct {
	Policies           []Policy            `json:"policies"`
	GlobalValues       map[string]string   `json:"globalValues"`
	NamespaceSelectors []NamespaceSelector `json:"namespaceSelector"`
	Subresources       []Subresource       `json:"subresources"`
}

type Resource struct {
	Name   string                 `json:"name"`
	Values map[string]interface{} `json:"values"`
}

type Subresource struct {
	APIResource    metav1.APIResource `json:"subresource"`
	ParentResource metav1.APIResource `json:"parentResource"`
}

type NamespaceSelector struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

type ApplyPolicyConfig struct {
	Policy                    kyvernov1.PolicyInterface
	Resource                  *unstructured.Unstructured
	MutateLogPath             string
	MutateLogPathIsDir        bool
	Variables                 map[string]interface{}
	UserInfo                  kyvernov1beta1.RequestInfo
	PolicyReport              bool
	NamespaceSelectorMap      map[string]map[string]string
	Stdin                     bool
	Rc                        *ResultCounts
	PrintPatchResource        bool
	RuleToCloneSourceResource map[string]string
	Client                    dclient.Interface
	AuditWarn                 bool
	Subresources              []Subresource
}

// HasVariables - check for variables in the policy
func HasVariables(policy kyvernov1.PolicyInterface) [][]string {
	policyRaw, _ := json.Marshal(policy)
	matches := regex.RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches
}

// GetPolicies - Extracting the policies from multiple YAML
func GetPolicies(paths []string) (policies []kyvernov1.PolicyInterface, errors []error) {
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
			files, err := os.ReadDir(path)
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
				req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, path, nil)
				if err != nil {
					err := fmt.Errorf("failed to process %v: %v", path, err.Error())
					errors = append(errors, err)
					continue
				}
				resp, err := http.DefaultClient.Do(req)
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

				fileBytes, err = io.ReadAll(resp.Body)
				if err != nil {
					err := fmt.Errorf("failed to process %v: %v", path, err.Error())
					errors = append(errors, err)
					continue
				}
			} else {
				path = filepath.Clean(path)
				// We accept the risk of including a user provided file here.
				fileBytes, err = os.ReadFile(path) // #nosec G304
				if err != nil {
					err := fmt.Errorf("failed to process %v: %v", path, err.Error())
					errors = append(errors, err)
					continue
				}
			}

			policiesFromFile, errFromFile := yamlutils.GetPolicy(fileBytes)
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
				if !strings.Contains(v, "request.object") && !strings.Contains(v, "element") && v == "elementIndex" {
					variableStr = variableStr + " " + v
				}
			}
		}
	}
	return variableStr
}

func GetVariable(variablesString, valuesFile string, fs billy.Filesystem, isGit bool, policyResourcePath string) (map[string]string, map[string]string, map[string]map[string]Resource, map[string]map[string]string, []Subresource, error) {
	valuesMapResource := make(map[string]map[string]Resource)
	valuesMapRule := make(map[string]map[string]Rule)
	namespaceSelectorMap := make(map[string]map[string]string)
	variables := make(map[string]string)
	subresources := make([]Subresource, 0)
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
			yamlFile, err = io.ReadAll(filep)
			if err != nil {
				fmt.Printf("Unable to read variable files: %s. error: %s \n", filep, err)
			}
		} else {
			// We accept the risk of including a user provided file here.
			yamlFile, err = os.ReadFile(filepath.Join(policyResourcePath, valuesFile)) // #nosec G304
			if err != nil {
				fmt.Printf("\n Unable to open variable file: %s. error: %s \n", valuesFile, err)
			}
		}

		if err != nil {
			return variables, globalValMap, valuesMapResource, namespaceSelectorMap, subresources, sanitizederror.NewWithError("unable to read yaml", err)
		}

		valuesBytes, err := yaml.ToJSON(yamlFile)
		if err != nil {
			return variables, globalValMap, valuesMapResource, namespaceSelectorMap, subresources, sanitizederror.NewWithError("failed to convert json", err)
		}

		values := &Values{}
		if err := json.Unmarshal(valuesBytes, values); err != nil {
			return variables, globalValMap, valuesMapResource, namespaceSelectorMap, subresources, sanitizederror.NewWithError("failed to decode yaml", err)
		}

		if values.GlobalValues == nil {
			values.GlobalValues = make(map[string]string)
			values.GlobalValues["request.operation"] = "CREATE"
			log.Log.V(3).Info("Defaulting request.operation to CREATE")
		} else {
			if val, ok := values.GlobalValues["request.operation"]; ok {
				if val == "" {
					values.GlobalValues["request.operation"] = "CREATE"
					log.Log.V(3).Info("Globally request.operation value provided by the user is empty, defaulting it to CREATE", "request.opearation: ", values.GlobalValues)
				}
			}
		}

		globalValMap = values.GlobalValues

		for _, p := range values.Policies {
			resourceMap := make(map[string]Resource)
			for _, r := range p.Resources {
				if val, ok := r.Values["request.operation"]; ok {
					if val == "" {
						r.Values["request.operation"] = "CREATE"
						log.Log.V(3).Info("No request.operation found, defaulting it to CREATE", "policy", p.Name)
					}
				}
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

		subresources = values.Subresources
	}

	if reqObjVars != "" {
		fmt.Printf("\nNOTICE: request.object.* variables are automatically parsed from the supplied resource. Ignoring value of variables `%v`.\n", reqObjVars)
	}

	if globalValMap != nil {
		if _, ok := globalValMap["request.operation"]; !ok {
			globalValMap["request.operation"] = "CREATE"
			log.Log.V(3).Info("Defaulting request.operation to CREATE")
		}
	}

	storePolicies := make([]store.Policy, 0)
	for policyName, ruleMap := range valuesMapRule {
		storeRules := make([]store.Rule, 0)
		for _, rule := range ruleMap {
			storeRules = append(storeRules, store.Rule{
				Name:          rule.Name,
				Values:        rule.Values,
				ForEachValues: rule.ForeachValues,
			})
		}
		storePolicies = append(storePolicies, store.Policy{
			Name:  policyName,
			Rules: storeRules,
		})
	}

	store.SetPolicies(storePolicies...)

	return variables, globalValMap, valuesMapResource, namespaceSelectorMap, subresources, nil
}

// ApplyPolicyOnResource - function to apply policy on resource
func ApplyPolicyOnResource(c ApplyPolicyConfig) ([]*engineapi.EngineResponse, Info, error) {
	var engineResponses []*engineapi.EngineResponse
	namespaceLabels := make(map[string]string)
	operationIsDelete := false
	jp := jmespath.New(config.NewDefaultConfiguration(false))

	if c.Variables["request.operation"] == "DELETE" {
		operationIsDelete = true
	}

	policyWithNamespaceSelector := false
OuterLoop:
	for _, p := range autogen.ComputeRules(c.Policy) {
		if p.MatchResources.ResourceDescription.NamespaceSelector != nil ||
			p.ExcludeResources.ResourceDescription.NamespaceSelector != nil {
			policyWithNamespaceSelector = true
			break
		}
		for _, m := range p.MatchResources.Any {
			if m.ResourceDescription.NamespaceSelector != nil {
				policyWithNamespaceSelector = true
				break OuterLoop
			}
		}
		for _, m := range p.MatchResources.All {
			if m.ResourceDescription.NamespaceSelector != nil {
				policyWithNamespaceSelector = true
				break OuterLoop
			}
		}
		for _, e := range p.ExcludeResources.Any {
			if e.ResourceDescription.NamespaceSelector != nil {
				policyWithNamespaceSelector = true
				break OuterLoop
			}
		}
		for _, e := range p.ExcludeResources.All {
			if e.ResourceDescription.NamespaceSelector != nil {
				policyWithNamespaceSelector = true
				break OuterLoop
			}
		}
	}

	if policyWithNamespaceSelector {
		resourceNamespace := c.Resource.GetNamespace()
		namespaceLabels = c.NamespaceSelectorMap[c.Resource.GetNamespace()]
		if resourceNamespace != "default" && len(namespaceLabels) < 1 {
			return engineResponses, Info{}, sanitizederror.NewWithError(fmt.Sprintf("failed to get namespace labels for resource %s. use --values-file flag to pass the namespace labels", c.Resource.GetName()), nil)
		}
	}

	resPath := fmt.Sprintf("%s/%s/%s", c.Resource.GetNamespace(), c.Resource.GetKind(), c.Resource.GetName())
	log.Log.V(3).Info("applying policy on resource", "policy", c.Policy.GetName(), "resource", resPath)

	resourceRaw, err := c.Resource.MarshalJSON()
	if err != nil {
		log.Log.Error(err, "failed to marshal resource")
	}

	updatedResource, err := kubeutils.BytesToUnstructured(resourceRaw)
	if err != nil {
		log.Log.Error(err, "unable to convert raw resource to unstructured")
	}
	ctx := engineContext.NewContext(jp)

	if operationIsDelete {
		err = engineContext.AddOldResource(ctx, resourceRaw)
	} else {
		err = engineContext.AddResource(ctx, resourceRaw)
	}

	if err != nil {
		log.Log.Error(err, "failed to load resource in context")
	}

	for key, value := range c.Variables {
		err = ctx.AddVariable(key, value)
		if err != nil {
			log.Log.Error(err, "failed to add variable to context")
		}
	}

	cfg := config.NewDefaultConfiguration(false)
	if err := ctx.AddImageInfos(c.Resource, cfg); err != nil {
		log.Log.Error(err, "failed to add image variables to context")
	}

	gvk, subresource := updatedResource.GroupVersionKind(), ""
	// If --cluster flag is not set, then we need to find the top level resource GVK and subresource
	if c.Client == nil {
		for _, s := range c.Subresources {
			subgvk := schema.GroupVersionKind{
				Group:   s.APIResource.Group,
				Version: s.APIResource.Version,
				Kind:    s.APIResource.Kind,
			}
			if gvk == subgvk {
				gvk = schema.GroupVersionKind{
					Group:   s.ParentResource.Group,
					Version: s.ParentResource.Version,
					Kind:    s.ParentResource.Kind,
				}
				parts := strings.Split(s.APIResource.Name, "/")
				subresource = parts[1]
			}
		}
	}
	eng := engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jmespath.New(cfg),
		c.Client,
		registryclient.NewOrDie(),
		store.ContextLoaderFactory(nil),
		nil,
	)
	policyContext := engine.NewPolicyContextWithJsonContext(kyvernov1.Create, ctx).
		WithPolicy(c.Policy).
		WithNewResource(*updatedResource).
		WithNamespaceLabels(namespaceLabels).
		WithAdmissionInfo(c.UserInfo).
		WithResourceKind(gvk, subresource)

	mutateResponse := eng.Mutate(context.Background(), policyContext)
	engineResponses = append(engineResponses, &mutateResponse)

	err = processMutateEngineResponse(c, &mutateResponse, resPath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return engineResponses, Info{}, sanitizederror.NewWithError("failed to print mutated result", err)
		}
	}

	var policyHasValidate bool
	for _, rule := range autogen.ComputeRules(c.Policy) {
		if rule.HasValidate() || rule.HasVerifyImageChecks() {
			policyHasValidate = true
		}
	}

	policyContext = policyContext.WithNewResource(mutateResponse.PatchedResource)

	var info Info
	var validateResponse engineapi.EngineResponse
	if policyHasValidate {
		validateResponse = eng.Validate(context.Background(), policyContext)
		info = ProcessValidateEngineResponse(c.Policy, &validateResponse, resPath, c.Rc, c.PolicyReport, c.AuditWarn)
	}

	if !validateResponse.IsEmpty() {
		engineResponses = append(engineResponses, &validateResponse)
	}

	verifyImageResponse, _ := eng.VerifyAndPatchImages(context.TODO(), policyContext)
	if !verifyImageResponse.IsEmpty() {
		engineResponses = append(engineResponses, &verifyImageResponse)
		info = ProcessValidateEngineResponse(c.Policy, &verifyImageResponse, resPath, c.Rc, c.PolicyReport, c.AuditWarn)
	}

	var policyHasGenerate bool
	for _, rule := range autogen.ComputeRules(c.Policy) {
		if rule.HasGenerate() {
			policyHasGenerate = true
		}
	}

	if policyHasGenerate {
		generateResponse := eng.ApplyBackgroundChecks(context.TODO(), policyContext)
		if !generateResponse.IsEmpty() {
			newRuleResponse, err := handleGeneratePolicy(&generateResponse, *policyContext, c.RuleToCloneSourceResource)
			if err != nil {
				log.Log.Error(err, "failed to apply generate policy")
			} else {
				generateResponse.PolicyResponse.Rules = newRuleResponse
			}
			engineResponses = append(engineResponses, &generateResponse)
		}
		updateResultCounts(c.Policy, &generateResponse, resPath, c.Rc, c.AuditWarn)
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
		f, err = os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304
	} else {
		f, err = os.OpenFile(mutateLogPath+"/"+fileName+".yaml", os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304
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
func GetPoliciesFromPaths(fs billy.Filesystem, dirPath []string, isGit bool, policyResourcePath string) (policies []kyvernov1.PolicyInterface, err error) {
	if isGit {
		for _, pp := range dirPath {
			filep, err := fs.Open(filepath.Join(policyResourcePath, pp))
			if err != nil {
				fmt.Printf("Error: file not available with path %s: %v", filep.Name(), err.Error())
				continue
			}
			bytes, err := io.ReadAll(filep)
			if err != nil {
				fmt.Printf("Error: failed to read file %s: %v", filep.Name(), err.Error())
				continue
			}
			policyBytes, err := yaml.ToJSON(bytes)
			if err != nil {
				fmt.Printf("failed to convert to JSON: %v", err)
				continue
			}
			policiesFromFile, errFromFile := yamlutils.GetPolicy(policyBytes)
			if errFromFile != nil {
				fmt.Printf("failed to process : %v", errFromFile.Error())
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
				policies, err = yamlutils.GetPolicy(yamlBytes)
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
	cluster bool, policies []kyvernov1.PolicyInterface, dClient dclient.Interface, namespace string, policyReport bool, isGit bool, policyResourcePath string,
) (resources []*unstructured.Unstructured, err error) {
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
		} else {
			if len(resourcePaths) > 0 {
				fileDesc, err := os.Stat(resourcePaths[0])
				if err != nil {
					return nil, err
				}
				if fileDesc.IsDir() {
					files, err := os.ReadDir(resourcePaths[0])
					if err != nil {
						return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to parse %v", resourcePaths[0]), err)
					}
					listOfFiles := make([]string, 0)
					for _, file := range files {
						ext := filepath.Ext(file.Name())
						if ext == ".yaml" || ext == ".yml" {
							listOfFiles = append(listOfFiles, filepath.Join(resourcePaths[0], file.Name()))
						}
					}
					resourcePaths = listOfFiles
				}
			}

			resources, err = GetResources(policies, resourcePaths, dClient, cluster, namespace, policyReport)
			if err != nil {
				return resources, err
			}
		}
	}
	return resources, err
}

func ProcessValidateEngineResponse(policy kyvernov1.PolicyInterface, validateResponse *engineapi.EngineResponse, resPath string, rc *ResultCounts, policyReport bool, auditWarn bool) Info {
	var violatedRules []kyvernov1.ViolatedRule

	printCount := 0
	for _, policyRule := range autogen.ComputeRules(policy) {
		ruleFoundInEngineResponse := false
		if !policyRule.HasValidate() && !policyRule.HasVerifyImageChecks() && !policyRule.HasVerifyImages() {
			continue
		}

		for i, valResponseRule := range validateResponse.PolicyResponse.Rules {
			if policyRule.Name == valResponseRule.Name() {
				ruleFoundInEngineResponse = true
				vrule := kyvernov1.ViolatedRule{
					Name:    valResponseRule.Name(),
					Type:    string(valResponseRule.RuleType()),
					Message: valResponseRule.Message(),
				}

				switch valResponseRule.Status() {
				case engineapi.RuleStatusPass:
					rc.Pass++
					vrule.Status = policyreportv1alpha2.StatusPass

				case engineapi.RuleStatusFail:
					auditWarning := false
					ann := policy.GetAnnotations()
					if scored, ok := ann[kyvernov1.AnnotationPolicyScored]; ok && scored == "false" {
						rc.Warn++
						vrule.Status = policyreportv1alpha2.StatusWarn
						break
					} else if auditWarn && validateResponse.GetValidationFailureAction().Audit() {
						rc.Warn++
						auditWarning = true
						vrule.Status = policyreportv1alpha2.StatusWarn
					} else {
						rc.Fail++
						vrule.Status = policyreportv1alpha2.StatusFail
					}

					if !policyReport {
						if printCount < 1 {
							if auditWarning {
								fmt.Printf("\npolicy %s -> resource %s failed as audit warning: \n", policy.GetName(), resPath)
							} else {
								fmt.Printf("\npolicy %s -> resource %s failed: \n", policy.GetName(), resPath)
							}
							printCount++
						}

						fmt.Printf("%d. %s: %s \n", i+1, valResponseRule.Name(), valResponseRule.Message())
					}

				case engineapi.RuleStatusError:
					rc.Error++
					vrule.Status = policyreportv1alpha2.StatusError

				case engineapi.RuleStatusWarn:
					rc.Warn++
					vrule.Status = policyreportv1alpha2.StatusWarn

				case engineapi.RuleStatusSkip:
					rc.Skip++
					vrule.Status = policyreportv1alpha2.StatusSkip
				}

				violatedRules = append(violatedRules, vrule)
				continue
			}
		}

		if !ruleFoundInEngineResponse {
			rc.Skip++
			vruleSkip := kyvernov1.ViolatedRule{
				Name:    policyRule.Name,
				Type:    "Validation",
				Message: policyRule.Validation.Message,
				Status:  policyreportv1alpha2.StatusSkip,
			}
			violatedRules = append(violatedRules, vruleSkip)
		}
	}
	return buildPVInfo(validateResponse, violatedRules)
}

func buildPVInfo(er *engineapi.EngineResponse, violatedRules []kyvernov1.ViolatedRule) Info {
	info := Info{
		PolicyName: er.Policy().GetName(),
		Namespace:  er.PatchedResource.GetNamespace(),
		Results: []EngineResponseResult{
			{
				Resource: er.GetResourceSpec(),
				Rules:    violatedRules,
			},
		},
	}
	return info
}

func updateResultCounts(policy kyvernov1.PolicyInterface, engineResponse *engineapi.EngineResponse, resPath string, rc *ResultCounts, auditWarn bool) {
	printCount := 0
	for _, policyRule := range autogen.ComputeRules(policy) {
		ruleFoundInEngineResponse := false
		for i, ruleResponse := range engineResponse.PolicyResponse.Rules {
			if policyRule.Name == ruleResponse.Name() {
				ruleFoundInEngineResponse = true

				if ruleResponse.Status() == engineapi.RuleStatusPass {
					rc.Pass++
				} else {
					if printCount < 1 {
						fmt.Println("\ninvalid resource", "policy", policy.GetName(), "resource", resPath)
						printCount++
					}
					fmt.Printf("%d. %s - %s\n", i+1, ruleResponse.Name(), ruleResponse.Message())

					if auditWarn && engineResponse.GetValidationFailureAction().Audit() {
						rc.Warn++
					} else {
						rc.Fail++
					}
				}
				continue
			}
		}

		if !ruleFoundInEngineResponse {
			rc.Skip++
		}
	}
}

func SetInStoreContext(mutatedPolicies []kyvernov1.PolicyInterface, variables map[string]string) map[string]string {
	storePolicies := make([]store.Policy, 0)
	for _, policy := range mutatedPolicies {
		storeRules := make([]store.Rule, 0)
		for _, rule := range autogen.ComputeRules(policy) {
			contextVal := make(map[string]interface{})
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
		storePolicies = append(storePolicies, store.Policy{
			Name:  policy.GetName(),
			Rules: storeRules,
		})
	}

	store.SetPolicies(storePolicies...)

	return variables
}

func processMutateEngineResponse(c ApplyPolicyConfig, mutateResponse *engineapi.EngineResponse, resPath string) error {
	var policyHasMutate bool
	for _, rule := range autogen.ComputeRules(c.Policy) {
		if rule.HasMutate() {
			policyHasMutate = true
		}
	}
	if !policyHasMutate {
		return nil
	}

	printCount := 0
	printMutatedRes := false
	for _, policyRule := range autogen.ComputeRules(c.Policy) {
		ruleFoundInEngineResponse := false
		for i, mutateResponseRule := range mutateResponse.PolicyResponse.Rules {
			if policyRule.Name == mutateResponseRule.Name() {
				ruleFoundInEngineResponse = true
				if mutateResponseRule.Status() == engineapi.RuleStatusPass {
					c.Rc.Pass++
					printMutatedRes = true
				} else if mutateResponseRule.Status() == engineapi.RuleStatusSkip {
					fmt.Printf("\nskipped mutate policy %s -> resource %s", c.Policy.GetName(), resPath)
					c.Rc.Skip++
				} else if mutateResponseRule.Status() == engineapi.RuleStatusError {
					fmt.Printf("\nerror while applying mutate policy %s -> resource %s\nerror: %s", c.Policy.GetName(), resPath, mutateResponseRule.Message())
					c.Rc.Error++
				} else {
					if printCount < 1 {
						fmt.Printf("\nfailed to apply mutate policy %s -> resource %s", c.Policy.GetName(), resPath)
						printCount++
					}
					fmt.Printf("%d. %s - %s \n", i+1, mutateResponseRule.Name(), mutateResponseRule.Message())
					c.Rc.Fail++
				}
				continue
			}
		}
		if !ruleFoundInEngineResponse {
			c.Rc.Skip++
		}
	}

	if printMutatedRes && c.PrintPatchResource {
		yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
		if err != nil {
			return sanitizederror.NewWithError("failed to marshal", err)
		}

		if c.MutateLogPath == "" {
			mutatedResource := string(yamlEncodedResource) + string("\n---")
			if len(strings.TrimSpace(mutatedResource)) > 0 {
				if !c.Stdin {
					fmt.Printf("\nmutate policy %s applied to %s:", c.Policy.GetName(), resPath)
				}
				fmt.Printf("\n" + mutatedResource + "\n")
			}
		} else {
			err := PrintMutatedOutput(c.MutateLogPath, c.MutateLogPathIsDir, string(yamlEncodedResource), c.Resource.GetName()+"-mutated")
			if err != nil {
				return sanitizederror.NewWithError("failed to print mutated result", err)
			}
			fmt.Printf("\n\nMutation:\nMutation has been applied successfully. Check the files.")
		}
	}

	return nil
}

func PrintMutatedPolicy(mutatedPolicies []kyvernov1.PolicyInterface) error {
	for _, policy := range mutatedPolicies {
		p, err := json.Marshal(policy)
		if err != nil {
			return sanitizederror.NewWithError("failed to marsal mutated policy", err)
		}
		log.Log.V(5).Info("mutated Policy:", string(p))
	}
	return nil
}

func CheckVariableForPolicy(valuesMap map[string]map[string]Resource, globalValMap map[string]string, policyName string, resourceName string, resourceKind string, variables map[string]string, kindOnwhichPolicyIsApplied map[string]struct{}, variable string) (map[string]interface{}, error) {
	// get values from file for this policy resource combination
	thisPolicyResourceValues := make(map[string]interface{})
	if len(valuesMap[policyName]) != 0 && !datautils.DeepEqual(valuesMap[policyName][resourceName], Resource{}) {
		thisPolicyResourceValues = valuesMap[policyName][resourceName].Values
	}

	for k, v := range variables {
		thisPolicyResourceValues[k] = v
	}

	if thisPolicyResourceValues == nil && len(globalValMap) > 0 {
		thisPolicyResourceValues = make(map[string]interface{})
	}

	for k, v := range globalValMap {
		if _, ok := thisPolicyResourceValues[k]; !ok {
			thisPolicyResourceValues[k] = v
		}
	}

	// skipping the variable check for non matching kind
	if _, ok := kindOnwhichPolicyIsApplied[resourceKind]; ok {
		if len(variable) > 0 && len(thisPolicyResourceValues) == 0 && store.HasPolicies() {
			return thisPolicyResourceValues, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policyName, resourceName), nil)
		}
	}
	return thisPolicyResourceValues, nil
}

func GetKindsFromPolicy(policy kyvernov1.PolicyInterface, subresources []Subresource, dClient dclient.Interface) map[string]struct{} {
	kindOnwhichPolicyIsApplied := make(map[string]struct{})
	for _, rule := range autogen.ComputeRules(policy) {
		for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
			k, err := getKind(kind, subresources, dClient)
			if err != nil {
				fmt.Printf("Error: %s", err.Error())
				continue
			}
			kindOnwhichPolicyIsApplied[k] = struct{}{}
		}
		for _, kind := range rule.ExcludeResources.ResourceDescription.Kinds {
			k, err := getKind(kind, subresources, dClient)
			if err != nil {
				fmt.Printf("Error: %s", err.Error())
				continue
			}
			kindOnwhichPolicyIsApplied[k] = struct{}{}
		}
	}
	return kindOnwhichPolicyIsApplied
}

func getKind(kind string, subresources []Subresource, dClient dclient.Interface) (string, error) {
	group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
	if subresource == "" {
		return kind, nil
	}
	if dClient == nil {
		gv := schema.GroupVersion{Group: group, Version: version}
		return getSubresourceKind(gv.String(), kind, subresource, subresources)
	}
	gvrss, err := dClient.Discovery().FindResources(group, version, kind, subresource)
	if err != nil {
		return kind, err
	}
	if len(gvrss) != 1 {
		return kind, fmt.Errorf("no unique match for kind %s", kind)
	}
	for _, api := range gvrss {
		return api.Kind, nil
	}
	return kind, nil
}

func getSubresourceKind(groupVersion, parentKind, subresourceName string, subresources []Subresource) (string, error) {
	for _, subresource := range subresources {
		parentResourceGroupVersion := metav1.GroupVersion{
			Group:   subresource.ParentResource.Group,
			Version: subresource.ParentResource.Version,
		}.String()
		if groupVersion == "" || kubeutils.GroupVersionMatches(groupVersion, parentResourceGroupVersion) {
			if parentKind == subresource.ParentResource.Kind {
				if strings.ToLower(subresourceName) == strings.Split(subresource.APIResource.Name, "/")[1] {
					return subresource.APIResource.Kind, nil
				}
			}
		}
	}
	return "", sanitizederror.NewWithError(fmt.Sprintf("subresource %s not found for parent resource %s", subresourceName, parentKind), nil)
}

// GetResourceFromPath - get patchedResource and generatedResource from given path
func GetResourceFromPath(fs billy.Filesystem, path string, isGit bool, policyResourcePath string, resourceType string) (unstructured.Unstructured, error) {
	var resourceBytes []byte
	var resource unstructured.Unstructured
	var err error
	if isGit {
		if len(path) > 0 {
			filep, fileErr := fs.Open(filepath.Join(policyResourcePath, path))
			if fileErr != nil {
				fmt.Printf("Unable to open %s file: %s. \nerror: %s", resourceType, path, err)
			}
			resourceBytes, err = io.ReadAll(filep)
		}
	} else {
		resourceBytes, err = getFileBytes(path)
	}

	if err != nil {
		fmt.Printf("\n----------------------------------------------------------------------\nfailed to load %s: %s. \nerror: %s\n----------------------------------------------------------------------\n", resourceType, path, err)
		return resource, err
	}

	resource, err = GetPatchedAndGeneratedResource(resourceBytes)
	if err != nil {
		return resource, err
	}

	return resource, nil
}

// initializeMockController initializes a basic Generate Controller with a fake dynamic client.
func initializeMockController(objects []runtime.Object) (*generate.GenerateController, error) {
	client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, objects...)
	if err != nil {
		fmt.Printf("Failed to mock dynamic client")
		return nil, err
	}

	client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	cfg := config.NewDefaultConfiguration(false)
	c := generate.NewGenerateControllerWithOnlyClient(client, engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jmespath.New(cfg),
		client,
		nil,
		store.ContextLoaderFactory(nil),
		nil,
	))
	return c, nil
}

// handleGeneratePolicy returns a new RuleResponse with the Kyverno generated resource configuration by applying the generate rule.
func handleGeneratePolicy(generateResponse *engineapi.EngineResponse, policyContext engine.PolicyContext, ruleToCloneSourceResource map[string]string) ([]engineapi.RuleResponse, error) {
	resource := policyContext.NewResource()
	objects := []runtime.Object{&resource}
	resources := []*unstructured.Unstructured{}
	for _, rule := range generateResponse.PolicyResponse.Rules {
		if path, ok := ruleToCloneSourceResource[rule.Name()]; ok {
			resourceBytes, err := getFileBytes(path)
			if err != nil {
				fmt.Printf("failed to get resource bytes\n")
			} else {
				resources, err = GetResource(resourceBytes)
				if err != nil {
					fmt.Printf("failed to convert resource bytes to unstructured format\n")
				}
			}
		}
	}

	for _, res := range resources {
		objects = append(objects, res)
	}

	c, err := initializeMockController(objects)
	if err != nil {
		fmt.Println("error at controller")
		return nil, err
	}

	gr := kyvernov1beta1.UpdateRequest{
		Spec: kyvernov1beta1.UpdateRequestSpec{
			Type:   kyvernov1beta1.Generate,
			Policy: generateResponse.Policy().GetName(),
			Resource: kyvernov1.ResourceSpec{
				Kind:       generateResponse.Resource.GetKind(),
				Namespace:  generateResponse.Resource.GetNamespace(),
				Name:       generateResponse.Resource.GetName(),
				APIVersion: generateResponse.Resource.GetAPIVersion(),
			},
		},
	}

	var newRuleResponse []engineapi.RuleResponse

	for _, rule := range generateResponse.PolicyResponse.Rules {
		genResource, err := c.ApplyGeneratePolicy(log.Log, &policyContext, gr, []string{rule.Name()})
		if err != nil {
			return nil, err
		}

		if genResource != nil {
			unstrGenResource, err := c.GetUnstrResource(genResource[0])
			if err != nil {
				return nil, err
			}
			newRuleResponse = append(newRuleResponse, *rule.WithGeneratedResource(*unstrGenResource))
		}
	}

	return newRuleResponse, nil
}

// GetUserInfoFromPath - get the request info as user info from a given path
func GetUserInfoFromPath(fs billy.Filesystem, path string, isGit bool, policyResourcePath string) (kyvernov1beta1.RequestInfo, error) {
	userInfo := &kyvernov1beta1.RequestInfo{}
	if isGit {
		filep, err := fs.Open(filepath.Join(policyResourcePath, path))
		if err != nil {
			fmt.Printf("Unable to open userInfo file: %s. \nerror: %s", path, err)
		}
		bytes, err := io.ReadAll(filep)
		if err != nil {
			fmt.Printf("Error: failed to read file %s: %v", filep.Name(), err.Error())
		}
		userInfoBytes, err := yaml.ToJSON(bytes)
		if err != nil {
			fmt.Printf("failed to convert to JSON: %v", err)
		}

		if err := json.Unmarshal(userInfoBytes, userInfo); err != nil {
			fmt.Printf("failed to decode yaml: %v", err)
		}
	} else {
		var errors []error
		pathname := filepath.Clean(filepath.Join(policyResourcePath, path))
		bytes, err := os.ReadFile(pathname)
		if err != nil {
			errors = append(errors, sanitizederror.NewWithError("unable to read yaml", err))
		}
		userInfoBytes, err := yaml.ToJSON(bytes)
		if err != nil {
			errors = append(errors, sanitizederror.NewWithError("failed to convert json", err))
		}
		if err := json.Unmarshal(userInfoBytes, userInfo); err != nil {
			errors = append(errors, sanitizederror.NewWithError("failed to decode yaml", err))
		}
		if len(errors) > 0 && log.Log.V(1).Enabled() {
			fmt.Printf("ignoring errors: \n")
			for _, e := range errors {
				fmt.Printf("    %v \n", e.Error())
			}
		}
	}
	return *userInfo, nil
}

func IsGitSourcePath(policyPaths []string) bool {
	return strings.Contains(policyPaths[0], "https://")
}

func GetGitBranchOrPolicyPaths(gitBranch, repoURL string, policyPaths []string) (string, string) {
	var gitPathToYamls string
	if gitBranch == "" {
		gitPathToYamls = "/"
		if string(policyPaths[0][len(policyPaths[0])-1]) == "/" {
			gitBranch = strings.ReplaceAll(policyPaths[0], repoURL+"/", "")
		} else {
			gitBranch = strings.ReplaceAll(policyPaths[0], repoURL, "")
		}
		if gitBranch == "" {
			gitBranch = "main"
		} else if string(gitBranch[0]) == "/" {
			gitBranch = gitBranch[1:]
		}
		return gitBranch, gitPathToYamls
	}
	if string(policyPaths[0][len(policyPaths[0])-1]) == "/" {
		gitPathToYamls = strings.ReplaceAll(policyPaths[0], repoURL+"/", "/")
	} else {
		gitPathToYamls = strings.ReplaceAll(policyPaths[0], repoURL, "/")
	}
	return gitBranch, gitPathToYamls
}
