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
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
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
	namespaceLabels := map[string]string{}
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
		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy, p.Variables.Subresources(), p.Client)
		resourceValues, err := p.Variables.ComputeVariables(policy.GetName(), resource.GetName(), resource.GetKind(), kindOnwhichPolicyIsApplied /*matches...*/)
		if err != nil {
			message := fmt.Sprintf(
				"policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag",
				policy.GetName(),
				resource.GetName(),
			)
			return nil, sanitizederror.NewWithError(message, err)
		}
		operation := kyvernov1.Create
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
		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy, p.Variables.Subresources(), p.Client)
		resourceValues, err := p.Variables.ComputeVariables(policy.GetName(), resource.GetName(), resource.GetKind(), kindOnwhichPolicyIsApplied /*matches...*/)
		if err != nil {
			message := fmt.Sprintf(
				"policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag",
				policy.GetName(),
				resource.GetName(),
			)
			return nil, sanitizederror.NewWithError(message, err)
		}
		operation := kyvernov1.Create
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
		// TODO annotation
		verifyImageResponse, _ := eng.VerifyAndPatchImages(context.TODO(), policyContext)
		if !verifyImageResponse.IsEmpty() {
			verifyImageResponse = combineRuleResponses(verifyImageResponse)
			responses = append(responses, verifyImageResponse)
			resource = verifyImageResponse.PatchedResource
		}
	}
	return responses, nil
}

func (p *PolicyProcessor) applyPolicyOnResource(policy kyvernov1.PolicyInterface) ([]engineapi.EngineResponse, error) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	resource := p.Resource
	var engineResponses []engineapi.EngineResponse
	namespaceLabels := make(map[string]string)
	operation := kyvernov1.Create

	// if p.Variables["request.operation"] == "DELETE" {
	// 	operation = kyvernov1.Delete
	// }
	rules := autogen.ComputeRules(policy)

	if needsNamespaceLabels(rules...) {
		resourceNamespace := p.Resource.GetNamespace()
		namespaceLabels = p.NamespaceSelectorMap[p.Resource.GetNamespace()]
		if resourceNamespace != "default" && len(namespaceLabels) < 1 {
			return engineResponses, sanitizederror.NewWithError(fmt.Sprintf("failed to get namespace labels for resource %s. use --values-file flag to pass the namespace labels", resource.GetName()), nil)
		}
	}

	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	log.Log.V(3).Info("applying policy on resource", "policy", policy.GetName(), "resource", resPath)

	resourceRaw, err := resource.MarshalJSON()
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
	policyContext, err := engine.NewPolicyContext(
		jp,
		*updatedResource,
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

	// for key, value := range p.Variables {
	// 	err = policyContext.JSONContext().AddVariable(key, value)
	// 	if err != nil {
	// 		log.Log.Error(err, "failed to add variable to context")
	// 	}
	// }

	mutateResponse := eng.Mutate(context.Background(), policyContext)
	combineRuleResponses(mutateResponse)
	engineResponses = append(engineResponses, mutateResponse)

	err = p.processMutateEngineResponse(mutateResponse, resPath)
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
	for _, rule := range rules {
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
	for _, rule := range rules {
		if rule.HasGenerate() {
			policyHasGenerate = true
		}
	}

	if policyHasGenerate {
		generateResponse := eng.ApplyBackgroundChecks(context.TODO(), policyContext)
		if !generateResponse.IsEmpty() {
			newRuleResponse, err := handleGeneratePolicy(&generateResponse, *policyContext, p.RuleToCloneSourceResource)
			if err != nil {
				log.Log.Error(err, "failed to apply generate policy")
			} else {
				generateResponse.PolicyResponse.Rules = newRuleResponse
			}
			combineRuleResponses(generateResponse)
			engineResponses = append(engineResponses, generateResponse)
		}
		p.Rc.addGenerateResponse(p.AuditWarn, resPath, generateResponse)
	}

	p.Rc.addEngineResponses(p.AuditWarn, engineResponses...)

	return engineResponses, nil
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
