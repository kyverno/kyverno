package mutation

import (
	"context"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type handler struct {
	configuration config.Configuration
	contextLoader func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.EngineContextLoader
}

func NewHandler(
	configuration config.Configuration,
	contextLoader func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.EngineContextLoader,
) handlers.Handler {
	return handler{
		configuration: configuration,
		contextLoader: contextLoader,
	}
}

func (h handler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	polexFilter func(logr.Logger, engineapi.PolicyContext, kyvernov1.Rule) *engineapi.RuleResponse,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	policy := policyContext.Policy()
	contextLoader := h.contextLoader(policy, rule)
	var excludeResource []string
	if len(h.configuration.GetExcludedGroups()) > 0 {
		excludeResource = h.configuration.GetExcludedGroups()
	}
	gvk, subresource := policyContext.ResourceKind()
	if err := engineutils.MatchesResourceDescription(
		resource,
		rule,
		policyContext.AdmissionInfo(),
		excludeResource,
		policyContext.NamespaceLabels(),
		policyContext.Policy().GetNamespace(),
		gvk,
		subresource,
	); err != nil {
		logger.V(4).Info("rule not matched", "reason", err.Error())
		return resource, nil
	}

	// check if there is a corresponding policy exception
	if ruleResp := polexFilter(logger, policyContext, rule); ruleResp != nil {
		return resource, handlers.RuleResponses(ruleResp)
	}

	logger.V(3).Info("processing mutate rule")

	if err := contextLoader(ctx, rule.Context, policyContext.JSONContext()); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			logger.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			logger.Error(err, "failed to load context")
		}
		// TODO: return error ?
		return resource, nil
	}
	var parentResourceGVR metav1.GroupVersionResource
	if subresource != "" {
		parentResourceGVR = policyContext.RequestResource()
	}

	resourceInfo := resourceInfo{
		unstructured:      resource,
		subresource:       subresource,
		parentResourceGVR: parentResourceGVR,
	}

	// logger.V(4).Info("apply rule to resource", "resource namespace", patchedResource.unstructured.GetNamespace(), "resource name", patchedResource.unstructured.GetName())
	var mutateResp *mutate.Response
	if rule.Mutation.ForEachMutation != nil {
		m := &forEachMutator{
			rule:          &rule,
			foreach:       rule.Mutation.ForEachMutation,
			policyContext: policyContext,
			resource:      resourceInfo,
			log:           logger,
			contextLoader: contextLoader,
			nesting:       0,
		}
		mutateResp = m.mutateForEach(ctx)
	} else {
		mutateResp = mutateResource(&rule, policyContext, resourceInfo.unstructured, logger)
	}
	if mutateResp == nil {
		return resource, nil
	}
	return mutateResp.PatchedResource, handlers.RuleResponses(buildRuleResponse(&rule, mutateResp, resourceInfo))
}
