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
		return resource, handlers.WithSkip(rule, engineapi.Mutation, "no patches")
	}
	// apply patchers
	resourceBytes, err := resource.MarshalJSON()
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return resource, handlers.WithError(rule, engineapi.Mutation, "failed to marshal resource", err)
	}
	var allPatches []jsonpatch.JsonPatchOperation
	for _, patcher := range patchers {
		patchedBytes, patches, err := patcher.Patch(logger, resourceBytes)
		if err != nil {
			logger.Error(err, "failed to patch resource")
			return resource, handlers.WithError(rule, engineapi.Mutation, "failed to patch resource", err)
		}
		resourceBytes = patchedBytes
		allPatches = append(allPatches, patches...)
	}
	if len(allPatches) == 0 {
		return resource, handlers.WithSkip(rule, engineapi.Mutation, "no patches")
	}
	err = resource.UnmarshalJSON(resourceBytes)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		return resource, handlers.WithError(rule, engineapi.Mutation, "failed to unmarshal resource", err)
	}
	resp := engineapi.RulePass(rule.Name, engineapi.Mutation, "TODO").WithPatches(patch.ConvertPatches(allPatches...)...)
	// if mutateResp.Status == engineapi.RuleStatusPass {
	// 	resp = resp
	// 	// TODO
	// 	// if len(rule.Mutation.Targets) != 0 {
	// 	// 	resp = resp.WithPatchedTarget(&mutateResp.PatchedResource, info.parentResourceGVR, info.subresource)
	// 	// }
	// }
	logger.Info("pass", "resp", resp)
	return resource, handlers.WithResponses(resp)
}
