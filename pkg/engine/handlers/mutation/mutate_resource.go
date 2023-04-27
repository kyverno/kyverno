package mutation

import (
	"context"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mutateResourceHandler struct{}

func NewMutateResourceHandler() (handlers.Handler, error) {
	return mutateResourceHandler{}, nil
}

func (h mutateResourceHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
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
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		// return *engineapi.RuleFail(ruleName, engineapi.Mutation, fmt.Sprintf("failed to marshal resource: %v", err)), resource
	}
	patch, err := jsonpatch.DecodePatch(jsonutils.JoinPatches(mutateResp.Patches...))
	if err != nil {
		// return resource, fmt.Errorf("failed to decode patches: %v", err)
	}
	patchedResourceRaw, err := patch.Apply(resourceRaw)
	if err != nil {
		// return resource, err
	}
	var patchedResource unstructured.Unstructured
	err = patchedResource.UnmarshalJSON(patchedResourceRaw)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		// return *engineapi.RuleFail(ruleName, engineapi.Mutation, fmt.Sprintf("failed to unmarshal resource: %v", err)), resource
	}
	return patchedResource, handlers.WithResponses(buildRuleResponse(&rule, mutateResp, resourceInfo))
}
