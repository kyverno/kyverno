package processor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy/annotations"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PolicyProcessor struct {
	Policy                    kyvernov1.PolicyInterface
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

func (c *PolicyProcessor) ApplyPolicyOnResource() ([]engineapi.EngineResponse, error) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))

	var engineResponses []engineapi.EngineResponse
	namespaceLabels := make(map[string]string)
	operation := kyvernov1.Create

	if c.Variables["request.operation"] == "DELETE" {
		operation = kyvernov1.Delete
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
			return engineResponses, sanitizederror.NewWithError(fmt.Sprintf("failed to get namespace labels for resource %s. use --values-file flag to pass the namespace labels", c.Resource.GetName()), nil)
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

	if err != nil {
		log.Log.Error(err, "failed to load resource in context")
	}

	cfg := config.NewDefaultConfiguration(false)
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
	var client engineapi.Client
	if c.Client != nil {
		client = adapters.Client(c.Client)
	}
	rclient := registryclient.NewOrDie()
	eng := engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jmespath.New(cfg),
		client,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		store.ContextLoaderFactory(nil),
		nil,
		"",
	)
	policyContext, err := engine.NewPolicyContext(
		jp,
		*updatedResource,
		operation,
		c.UserInfo,
		cfg,
	)
	if err != nil {
		log.Log.Error(err, "failed to create policy context")
	}

	policyContext = policyContext.
		WithPolicy(c.Policy).
		WithNamespaceLabels(namespaceLabels).
		WithResourceKind(gvk, subresource)

	for key, value := range c.Variables {
		err = policyContext.JSONContext().AddVariable(key, value)
		if err != nil {
			log.Log.Error(err, "failed to add variable to context")
		}
	}

	mutateResponse := eng.Mutate(context.Background(), policyContext)
	combineRuleResponses(mutateResponse)
	engineResponses = append(engineResponses, mutateResponse)

	err = c.processMutateEngineResponse(mutateResponse, resPath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return engineResponses, sanitizederror.NewWithError("failed to print mutated result", err)
		}
	}

	verifyImageResponse, _ := eng.VerifyAndPatchImages(context.TODO(), policyContext)
	if !verifyImageResponse.IsEmpty() {
		verifyImageResponse = combineRuleResponses(verifyImageResponse)
		engineResponses = append(engineResponses, verifyImageResponse)
	}

	var policyHasValidate bool
	for _, rule := range autogen.ComputeRules(c.Policy) {
		if rule.HasValidate() || rule.HasVerifyImageChecks() {
			policyHasValidate = true
		}
	}

	policyContext = policyContext.WithNewResource(mutateResponse.PatchedResource)

	var validateResponse engineapi.EngineResponse
	if policyHasValidate {
		validateResponse = eng.Validate(context.Background(), policyContext)
		validateResponse = combineRuleResponses(validateResponse)
	}

	if !validateResponse.IsEmpty() {
		engineResponses = append(engineResponses, validateResponse)
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
			combineRuleResponses(generateResponse)
			engineResponses = append(engineResponses, generateResponse)
		}
		updateResultCounts(c.Policy, &generateResponse, resPath, c.Rc, c.AuditWarn)
	}

	c.processEngineResponses(engineResponses...)

	return engineResponses, nil
}

func (c *PolicyProcessor) processEngineResponses(responses ...engineapi.EngineResponse) {
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

func (c *PolicyProcessor) processMutateEngineResponse(mutateResponse engineapi.EngineResponse, resourcePath string) error {
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
					fmt.Printf("\nskipped mutate policy %s -> resource %s", c.Policy.GetName(), resourcePath)
					c.Rc.Skip++
				} else if mutateResponseRule.Status() == engineapi.RuleStatusError {
					fmt.Printf("\nerror while applying mutate policy %s -> resource %s\nerror: %s", c.Policy.GetName(), resourcePath, mutateResponseRule.Message())
					c.Rc.Error++
				} else {
					if printCount < 1 {
						fmt.Printf("\nfailed to apply mutate policy %s -> resource %s", c.Policy.GetName(), resourcePath)
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
					fmt.Printf("\nmutate policy %s applied to %s:", c.Policy.GetName(), resourcePath)
				}
				fmt.Printf("\n" + mutatedResource + "\n")
			}
		} else {
			err := c.printMutatedOutput(string(yamlEncodedResource))
			if err != nil {
				return sanitizederror.NewWithError("failed to print mutated result", err)
			}
			fmt.Printf("\n\nMutation:\nMutation has been applied successfully. Check the files.")
		}
	}
	return nil
}

func (c *PolicyProcessor) printMutatedOutput(yaml string) error {
	var file *os.File
	mutateLogPath := filepath.Clean(c.MutateLogPath)
	filename := c.Resource.GetName() + "-mutated"
	if !c.MutateLogPathIsDir {
		// truncation for the case when mutateLogPath is a file (not a directory) is handled under pkg/kyverno/apply/test_command.go
		f, err := os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			return err
		}
		file = f
	} else {
		f, err := os.OpenFile(filepath.Join(mutateLogPath, filename+".yaml"), os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			return err
		}
		file = f
	}
	if _, err := file.Write([]byte(yaml + "\n---\n\n")); err != nil {
		if err := file.Close(); err != nil {
			log.Log.Error(err, "failed to close file")
		}
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}
