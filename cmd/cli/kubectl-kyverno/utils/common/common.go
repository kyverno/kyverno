package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/document"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy/annotations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResultCounts struct {
	Pass  int
	Fail  int
	Warn  int
	Error int
	Skip  int
}

type ApplyPolicyConfig struct {
	Policy                    kyvernov1.PolicyInterface
	ValidatingAdmissionPolicy v1alpha1.ValidatingAdmissionPolicy
	Resource                  *unstructured.Unstructured
	MutateLogPath             string
	MutateLogPathIsDir        bool
	Variables                 map[string]interface{}
	UserInfo                  *kyvernov1beta1.RequestInfo
	PolicyReport              bool
	NamespaceSelectorMap      map[string]map[string]string
	Stdin                     bool
	Rc                        *ResultCounts
	PrintPatchResource        bool
	RuleToCloneSourceResource map[string]string
	Client                    dclient.Interface
	AuditWarn                 bool
	Subresources              []valuesapi.Subresource
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

// GetResourceAccordingToResourcePath - get resources according to the resource path
func GetResourceAccordingToResourcePath(fs billy.Filesystem, resourcePaths []string,
	cluster bool, policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, dClient dclient.Interface, namespace string, policyReport bool, isGit bool, policyResourcePath string,
) (resources []*unstructured.Unstructured, err error) {
	if isGit {
		resources, err = GetResourcesWithTest(fs, policies, resourcePaths, isGit, policyResourcePath)
		if err != nil {
			return nil, sanitizederror.NewWithError("failed to extract the resources", err)
		}
	} else {
		if len(resourcePaths) > 0 && resourcePaths[0] == "-" {
			if document.IsStdin(resourcePaths[0]) {
				resourceStr := ""
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					resourceStr = resourceStr + scanner.Text() + "\n"
				}

				yamlBytes := []byte(resourceStr)
				resources, err = resource.GetUnstructuredResources(yamlBytes)
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

			resources, err = GetResources(policies, validatingAdmissionPolicies, resourcePaths, dClient, cluster, namespace, policyReport)
			if err != nil {
				return resources, err
			}
		}
	}
	return resources, err
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

func GetKindsFromPolicy(policy kyvernov1.PolicyInterface, subresources []valuesapi.Subresource, dClient dclient.Interface) map[string]struct{} {
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

func getKind(kind string, subresources []valuesapi.Subresource, dClient dclient.Interface) (string, error) {
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

func getSubresourceKind(groupVersion, parentKind, subresourceName string, subresources []valuesapi.Subresource) (string, error) {
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
		adapters.Client(client),
		nil,
		imageverifycache.DisabledImageVerifyCache(),
		store.ContextLoaderFactory(nil),
		nil,
		"",
	))
	return c, nil
}

// handleGeneratePolicy returns a new RuleResponse with the Kyverno generated resource configuration by applying the generate rule.
func handleGeneratePolicy(generateResponse *engineapi.EngineResponse, policyContext engine.PolicyContext, ruleToCloneSourceResource map[string]string) ([]engineapi.RuleResponse, error) {
	newResource := policyContext.NewResource()
	objects := []runtime.Object{&newResource}
	resources := []*unstructured.Unstructured{}
	for _, rule := range generateResponse.PolicyResponse.Rules {
		if path, ok := ruleToCloneSourceResource[rule.Name()]; ok {
			resourceBytes, err := resource.GetFileBytes(path)
			if err != nil {
				fmt.Printf("failed to get resource bytes\n")
			} else {
				resources, err = resource.GetUnstructuredResources(resourceBytes)
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
		genResource, err := c.ApplyGeneratePolicy(log.Log.V(2), &policyContext, gr, []string{rule.Name()})
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

func GetGitBranchOrPolicyPaths(gitBranch, repoURL string, policyPaths ...string) (string, string) {
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

func processEngineResponses(responses []engineapi.EngineResponse, c ApplyPolicyConfig) {
	for _, response := range responses {
		if !response.IsEmpty() {
			pol := response.Policy()
			if polType := pol.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
				return
			}
			scored := annotations.Scored(c.Policy.GetAnnotations())
			for _, rule := range autogen.ComputeRules(pol.GetPolicy().(kyvernov1.PolicyInterface)) {
				if rule.HasValidate() || rule.HasVerifyImageChecks() || rule.HasVerifyImages() {
					ruleFoundInEngineResponse := false
					for _, valResponseRule := range response.PolicyResponse.Rules {
						if rule.Name == valResponseRule.Name() {
							ruleFoundInEngineResponse = true
							switch valResponseRule.Status() {
							case engineapi.RuleStatusPass:
								c.Rc.Pass++
							case engineapi.RuleStatusFail:
								if !scored {
									c.Rc.Warn++
									break
								} else if c.AuditWarn && response.GetValidationFailureAction().Audit() {
									c.Rc.Warn++
								} else {
									c.Rc.Fail++
								}
							case engineapi.RuleStatusError:
								c.Rc.Error++
							case engineapi.RuleStatusWarn:
								c.Rc.Warn++
							case engineapi.RuleStatusSkip:
								c.Rc.Skip++
							}
							continue
						}
					}
					if !ruleFoundInEngineResponse {
						c.Rc.Skip++
					}
				}
			}
		}
	}
}
