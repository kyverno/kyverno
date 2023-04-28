package mutation

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/mattbaird/jsonpatch"
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
	var patchers []patch.Patcher
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
		p, err := m.mutateForEach(ctx)
		if err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to collect patchers", err),
			)
		}
		patchers = p
	} else {
		p, err := mutate.Mutate(logger, &rule, policyContext.JSONContext())
		if err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to collect patchers", err),
			)
		}
		patchers = append(patchers, p)
	}
	if len(patchers) == 0 {
		return resource, nil
	}
	// apply patchers
	resourceBytes, err := resource.MarshalJSON()
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		// return *engineapi.RuleFail(ruleName, engineapi.Mutation, fmt.Sprintf("failed to marshal resource: %v", err)), resource
	}
	var allPatches []jsonpatch.JsonPatchOperation
	for _, patcher := range patchers {
		patchedBytes, patches, err := patcher.Patch(logger, resourceBytes)
		if err != nil {
			// return resource, fmt.Errorf("failed to decode patches: %v", err)
		}
		resourceBytes = patchedBytes
		allPatches = append(allPatches, patches...)
	}
	// 	patch, err := jsonpatch.DecodePatch(jsonutils.JoinPatches([]byte(p.Json())))
	// 	if err != nil {
	// 		// return resource, fmt.Errorf("failed to decode patches: %v", err)
	// 	}
	// 	options := &jsonpatch.ApplyOptions{SupportNegativeIndices: true, AllowMissingPathOnRemove: true, EnsurePathExistsOnAdd: true}
	// 	patchedResourceRaw, err := patch.ApplyWithOptions(resourceRaw, options)
	// 	if err != nil {
	// 		// return resource, err
	// 	}
	// 	resourceRaw = patchedResourceRaw
	// }
	// var patchedResource unstructured.Unstructured
	err = resource.UnmarshalJSON(resourceBytes)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		// return *engineapi.RuleFail(ruleName, engineapi.Mutation, fmt.Sprintf("failed to unmarshal resource: %v", err)), resource
	}
	resp := engineapi.RulePass(rule.Name, engineapi.Mutation, "TODO").WithPatches(patch.ConvertPatches(allPatches...)...)
	// if mutateResp.Status == engineapi.RuleStatusPass {
	// 	resp = resp
	// 	// TODO
	// 	// if len(rule.Mutation.Targets) != 0 {
	// 	// 	resp = resp.WithPatchedTarget(&mutateResp.PatchedResource, info.parentResourceGVR, info.subresource)
	// 	// }
	// }
	return resource, handlers.WithResponses(resp)
}
