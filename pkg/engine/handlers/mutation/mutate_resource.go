package mutation

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mutateResourceHandler struct {
	contextLoader engineapi.EngineContextLoaderFactory
}

func NewMutateResourceHandler(
	contextLoader engineapi.EngineContextLoaderFactory,
) handlers.Handler {
	return mutateResourceHandler{
		contextLoader: contextLoader,
	}
}

func (h mutateResourceHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	policy := policyContext.Policy()
	contextLoader := h.contextLoader(policy, rule)
	_, subresource := policyContext.ResourceKind()
	logger.V(3).Info("processing mutate rule")
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
			rule:          rule,
			foreach:       rule.Mutation.ForEachMutation,
			policyContext: policyContext,
			resource:      resourceInfo,
			logger:        logger,
			contextLoader: contextLoader,
			nesting:       0,
		}
		mutateResp = m.mutateForEach(ctx)
	} else {
		mutateResp = mutate.Mutate(&rule, policyContext.JSONContext(), resource, logger)
	}
	if mutateResp == nil {
		return resource, nil
	}
	return mutateResp.PatchedResource, handlers.RuleResponses(buildRuleResponse(&rule, mutateResp, resourceInfo))
}
