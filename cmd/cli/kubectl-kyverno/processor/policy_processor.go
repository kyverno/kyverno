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
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PolicyProcessor struct {
	Policies                  []kyvernov1.PolicyInterface
	Resource                  unstructured.Unstructured
	MutateLogPath             string
	MutateLogPathIsDir        bool
	Variables                 *variables.Variables
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

func (p *PolicyProcessor) ApplyPoliciesOnResource() ([]engineapi.EngineResponse, error) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	resource := p.Resource
	namespaceLabels := p.NamespaceSelectorMap[p.Resource.GetNamespace()]
	var client engineapi.Client
	if p.Client != nil {
		client = adapters.Client(p.Client)
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
	gvk, subresource := resource.GroupVersionKind(), ""
	// If --cluster flag is not set, then we need to find the top level resource GVK and subresource
	if p.Client == nil {
		for _, s := range p.Subresources {
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
	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	var responses []engineapi.EngineResponse
	// mutate
	for _, policy := range p.Policies {
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		mutateResponse := eng.Mutate(context.Background(), policyContext)
		combineRuleResponses(mutateResponse)
		err = p.processMutateEngineResponse(mutateResponse, resPath)
		if err != nil {
			if !sanitizederror.IsErrorSanitized(err) {
				return responses, sanitizederror.NewWithError("failed to print mutated result", err)
			}
		}
		responses = append(responses, mutateResponse)
		resource = mutateResponse.PatchedResource
	}
	// verify images
	for _, policy := range p.Policies {
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		// TODO annotation
		verifyImageResponse, _ := eng.VerifyAndPatchImages(context.TODO(), policyContext)
		if !verifyImageResponse.IsEmpty() {
			verifyImageResponse = combineRuleResponses(verifyImageResponse)
			responses = append(responses, verifyImageResponse)
			resource = verifyImageResponse.PatchedResource
		}
	}
	// validate
	for _, policy := range p.Policies {
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		validateResponse := eng.Validate(context.TODO(), policyContext)
		if !validateResponse.IsEmpty() {
			validateResponse = combineRuleResponses(validateResponse)
			responses = append(responses, validateResponse)
			resource = validateResponse.PatchedResource
		}
	}
	// generate
	for _, policy := range p.Policies {
		var policyHasGenerate bool
		for _, rule := range autogen.ComputeRules(policy) {
			if rule.HasGenerate() {
				policyHasGenerate = true
			}
		}
		if policyHasGenerate {
			policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
			if err != nil {
				return responses, err
			}

			generateResponse := eng.ApplyBackgroundChecks(context.TODO(), policyContext)
			if !generateResponse.IsEmpty() {
				newRuleResponse, err := handleGeneratePolicy(&generateResponse, *policyContext, p.RuleToCloneSourceResource)
				if err != nil {
					log.Log.Error(err, "failed to apply generate policy")
				} else {
					generateResponse.PolicyResponse.Rules = newRuleResponse
				}
				combineRuleResponses(generateResponse)
				responses = append(responses, generateResponse)
			}
			p.Rc.addGenerateResponse(p.AuditWarn, resPath, generateResponse)
		}
	}
	p.Rc.addEngineResponses(p.AuditWarn, responses...)
	return responses, nil
}

func (p *PolicyProcessor) makePolicyContext(
	jp jmespath.Interface,
	cfg config.Configuration,
	resource unstructured.Unstructured,
	policy kyvernov1.PolicyInterface,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
) (*policycontext.PolicyContext, error) {
	operation := kyvernov1.Create
	var resourceValues map[string]interface{}
	if p.Variables != nil {
		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy, p.Variables.Subresources(), p.Client)
		vals, err := p.Variables.ComputeVariables(policy.GetName(), resource.GetName(), resource.GetKind(), kindOnwhichPolicyIsApplied /*matches...*/)
		if err != nil {
			message := fmt.Sprintf(
				"policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag",
				policy.GetName(),
				resource.GetName(),
			)
			return nil, sanitizederror.NewWithError(message, err)
		}
		resourceValues = vals
	}
	if resourceValues["request.operation"] == "DELETE" {
		operation = kyvernov1.Delete
	}
	policyContext, err := engine.NewPolicyContext(
		jp,
		resource,
		operation,
		p.UserInfo,
		cfg,
	)
	if err != nil {
		log.Log.Error(err, "failed to create policy context")
	}
	policyContext = policyContext.
		WithPolicy(policy).
		WithNamespaceLabels(namespaceLabels).
		WithResourceKind(gvk, subresource)
	for key, value := range resourceValues {
		err = policyContext.JSONContext().AddVariable(key, value)
		if err != nil {
			log.Log.Error(err, "failed to add variable to context")
		}
	}
	return policyContext, nil
}

func (p *PolicyProcessor) processMutateEngineResponse(response engineapi.EngineResponse, resourcePath string) error {
	printMutatedRes := p.Rc.addMutateResponse(resourcePath, response)
	if printMutatedRes && p.PrintPatchResource {
		yamlEncodedResource, err := yamlv2.Marshal(response.PatchedResource.Object)
		if err != nil {
			return sanitizederror.NewWithError("failed to marshal", err)
		}

		if p.MutateLogPath == "" {
			mutatedResource := string(yamlEncodedResource) + string("\n---")
			if len(strings.TrimSpace(mutatedResource)) > 0 {
				if !p.Stdin {
					fmt.Printf("\nmutate policy %s applied to %s:", response.Policy().GetName(), resourcePath)
				}
				fmt.Printf("\n" + mutatedResource + "\n")
			}
		} else {
			err := p.printMutatedOutput(string(yamlEncodedResource))
			if err != nil {
				return sanitizederror.NewWithError("failed to print mutated result", err)
			}
			fmt.Printf("\n\nMutation:\nMutation has been applied successfully. Check the files.")
		}
	}
	return nil
}

func (p *PolicyProcessor) printMutatedOutput(yaml string) error {
	var file *os.File
	mutateLogPath := filepath.Clean(p.MutateLogPath)
	filename := p.Resource.GetName() + "-mutated"
	if !p.MutateLogPathIsDir {
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
