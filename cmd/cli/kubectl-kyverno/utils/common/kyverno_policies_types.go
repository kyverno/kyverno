package common

import (
	"context"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ApplyPolicyOnResource - function to apply policy on resource
func ApplyPolicyOnResource(c ApplyPolicyConfig) ([]engineapi.EngineResponse, error) {
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
	log.V(3).Info("applying policy on resource", "policy", c.Policy.GetName(), "resource", resPath)

	resourceRaw, err := c.Resource.MarshalJSON()
	if err != nil {
		log.Error(err, "failed to marshal resource")
	}

	updatedResource, err := kubeutils.BytesToUnstructured(resourceRaw)
	if err != nil {
		log.Error(err, "unable to convert raw resource to unstructured")
	}

	if err != nil {
		log.Error(err, "failed to load resource in context")
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
	rclient := registryclient.NewOrDie()
	eng := engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jmespath.New(cfg),
		adapters.Client(c.Client),
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		store.ContextLoaderFactory(nil),
		nil,
		"",
	)
	policyContext, err := engine.NewPolicyContext(
		jp,
		*updatedResource,
		operation,
		&c.UserInfo,
		cfg,
	)
	if err != nil {
		log.Error(err, "failed to create policy context")
	}

	policyContext = policyContext.
		WithPolicy(c.Policy).
		WithNamespaceLabels(namespaceLabels).
		WithResourceKind(gvk, subresource)

	for key, value := range c.Variables {
		err = policyContext.JSONContext().AddVariable(key, value)
		if err != nil {
			log.Error(err, "failed to add variable to context")
		}
	}

	mutateResponse := eng.Mutate(context.Background(), policyContext)
	engineResponses = append(engineResponses, mutateResponse)

	err = processMutateEngineResponse(c, &mutateResponse, resPath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return engineResponses, sanitizederror.NewWithError("failed to print mutated result", err)
		}
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
		ProcessValidateEngineResponse(c.Policy, validateResponse, resPath, c.Rc, c.PolicyReport, c.AuditWarn)
	}

	if !validateResponse.IsEmpty() {
		engineResponses = append(engineResponses, validateResponse)
	}

	verifyImageResponse, _ := eng.VerifyAndPatchImages(context.TODO(), policyContext)
	if !verifyImageResponse.IsEmpty() {
		engineResponses = append(engineResponses, verifyImageResponse)
		ProcessValidateEngineResponse(c.Policy, verifyImageResponse, resPath, c.Rc, c.PolicyReport, c.AuditWarn)
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
				log.Error(err, "failed to apply generate policy")
			} else {
				generateResponse.PolicyResponse.Rules = newRuleResponse
			}
			engineResponses = append(engineResponses, generateResponse)
		}
		updateResultCounts(c.Policy, &generateResponse, resPath, c.Rc, c.AuditWarn)
	}

	return engineResponses, nil
}
