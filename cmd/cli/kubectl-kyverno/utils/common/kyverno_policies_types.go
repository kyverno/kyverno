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
	"github.com/kyverno/kyverno/pkg/imageverifycache"
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
		imageverifycache.DisabledImageVerifyCache(),
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
	combineRuleResponses(mutateResponse)
	engineResponses = append(engineResponses, mutateResponse)

	err = processMutateEngineResponse(c, &mutateResponse, resPath)
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
				log.Error(err, "failed to apply generate policy")
			} else {
				generateResponse.PolicyResponse.Rules = newRuleResponse
			}
			combineRuleResponses(generateResponse)
			engineResponses = append(engineResponses, generateResponse)
		}
		updateResultCounts(c.Policy, &generateResponse, resPath, c.Rc, c.AuditWarn)
	}

	processEngineResponses(engineResponses, c)

	return engineResponses, nil
}

func combineRuleResponses(imageResponse engineapi.EngineResponse) engineapi.EngineResponse {
	if imageResponse.PolicyResponse.RulesAppliedCount() == 0 {
		return imageResponse
	}

	completeRuleResponses := imageResponse.PolicyResponse.Rules
	var combineRuleResponses []engineapi.RuleResponse

	ruleNameType := make(map[string][]engineapi.RuleResponse)
	for _, rsp := range completeRuleResponses {
		key := rsp.Name() + ";" + string(rsp.RuleType())
		ruleNameType[key] = append(ruleNameType[key], rsp)
	}

	for key, ruleResponses := range ruleNameType {
		tokens := strings.Split(key, ";")
		ruleName := tokens[0]
		ruleType := tokens[1]
		var failRuleResponses []engineapi.RuleResponse
		var errorRuleResponses []engineapi.RuleResponse
		var passRuleResponses []engineapi.RuleResponse
		var skipRuleResponses []engineapi.RuleResponse

		ruleMesssage := ""
		for _, rsp := range ruleResponses {
			if rsp.Status() == engineapi.RuleStatusFail {
				failRuleResponses = append(failRuleResponses, rsp)
			} else if rsp.Status() == engineapi.RuleStatusError {
				errorRuleResponses = append(errorRuleResponses, rsp)
			} else if rsp.Status() == engineapi.RuleStatusPass {
				passRuleResponses = append(passRuleResponses, rsp)
			} else if rsp.Status() == engineapi.RuleStatusSkip {
				skipRuleResponses = append(skipRuleResponses, rsp)
			}
		}
		if len(errorRuleResponses) > 0 {
			for _, errRsp := range errorRuleResponses {
				ruleMesssage += errRsp.Message() + ";"
			}
			errorResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusError)
			combineRuleResponses = append(combineRuleResponses, *errorResponse)
			continue
		}

		if len(failRuleResponses) > 0 {
			for _, failRsp := range failRuleResponses {
				ruleMesssage += failRsp.Message() + ";"
			}
			failResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusFail)
			combineRuleResponses = append(combineRuleResponses, *failResponse)
			continue
		}

		if len(passRuleResponses) > 0 {
			for _, passRsp := range passRuleResponses {
				ruleMesssage += passRsp.Message() + ";"
			}
			passResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusPass)
			combineRuleResponses = append(combineRuleResponses, *passResponse)
			continue
		}

		for _, skipRsp := range skipRuleResponses {
			ruleMesssage += skipRsp.Message() + ";"
		}
		skipResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusSkip)
		combineRuleResponses = append(combineRuleResponses, *skipResponse)
	}
	imageResponse.PolicyResponse.Rules = combineRuleResponses
	return imageResponse
}
