package mutate

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Response struct {
	Status          engineapi.RuleStatus
	PatchedResource unstructured.Unstructured
	Message         string
}

func NewResponse(status engineapi.RuleStatus, resource unstructured.Unstructured, msg string) *Response {
	return &Response{
		Status:          status,
		PatchedResource: resource,
		Message:         msg,
	}
}

func NewErrorResponse(msg string, err error) *Response {
	if err != nil {
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	return NewResponse(engineapi.RuleStatusError, unstructured.Unstructured{}, msg)
}

func Mutate(rule *kyvernov1.Rule, ctx context.Interface, resource unstructured.Unstructured, logger logr.Logger) *Response {
	updatedRule, err := variables.SubstituteAllInRule(logger, ctx, *rule)
	if err != nil {
		return NewErrorResponse("variable substitution failed", err)
	}
	mutation := updatedRule.Mutation
	if mutation == nil {
		return NewErrorResponse("empty mutate rule", nil)
	}
	patcher := NewPatcher(mutation.GetPatchStrategicMerge(), mutation.PatchesJSON6902)
	if patcher == nil {
		return NewErrorResponse("empty mutate rule", nil)
	}

	patchedResource := resource.DeepCopy()
	resourceBytes, err := patchedResource.MarshalJSON()
	if err != nil {
		return NewErrorResponse("failed to marshal resource", err)
	}
	patchedBytes, err := patcher.Patch(logger, resourceBytes)
	if err != nil {
		return NewErrorResponse("failed to patch resource", err)
	}
	if strings.TrimSpace(string(resourceBytes)) == strings.TrimSpace(string(patchedBytes)) {
		return NewResponse(engineapi.RuleStatusSkip, resource, "no patches applied")
	}
	if err := patchedResource.UnmarshalJSON(patchedBytes); err != nil {
		return NewErrorResponse("failed to unmarshal patched resource", err)
	}
	if rule.HasMutateExisting() {
		if err := ctx.SetTargetResource(patchedResource.Object); err != nil {
			return NewErrorResponse("failed to update patched target resource in the JSON context", err)
		}
	}
	return NewResponse(engineapi.RuleStatusPass, *patchedResource, "resource patched")
}

func ForEach(name string, foreach kyvernov1.ForEachMutation, policyContext engineapi.PolicyContext, resource unstructured.Unstructured, element interface{}, logger logr.Logger) *Response {
	ctx := policyContext.JSONContext()
	fe, err := substituteAllInForEach(foreach, ctx, logger)
	if err != nil {
		return NewErrorResponse("variable substitution failed", err)
	}
	patcher := NewPatcher(fe["patchStrategicMerge"], fe["patchesJson6902"].(string))
	if patcher == nil {
		return NewErrorResponse("empty mutate rule", nil)
	}

	patchedResource := resource.DeepCopy()
	resourceBytes, err := patchedResource.MarshalJSON()
	if err != nil {
		return NewErrorResponse("failed to marshal resource", err)
	}
	patchedBytes, err := patcher.Patch(logger, resourceBytes)
	if err != nil {
		return NewErrorResponse("failed to patch resource", err)
	}
	if strings.TrimSpace(string(resourceBytes)) == strings.TrimSpace(string(patchedBytes)) {
		return NewResponse(engineapi.RuleStatusSkip, resource, "no patches applied")
	}
	if err := patchedResource.UnmarshalJSON(patchedBytes); err != nil {
		return NewErrorResponse("failed to unmarshal patched resource", err)
	}

	return NewResponse(engineapi.RuleStatusPass, *patchedResource, "resource patched")
}

func substituteAllInForEach(fe kyvernov1.ForEachMutation, ctx context.Interface, logger logr.Logger) (map[string]interface{}, error) {
	patchesMap := make(map[string]interface{})
	patchesMap["patchStrategicMerge"] = fe.GetPatchStrategicMerge()
	patchesMap["patchesJson6902"] = fe.PatchesJSON6902

	subedPatchesMap, err := variables.SubstituteAll(logger, ctx, patchesMap)
	if err != nil {
		return nil, err
	}

	typedMap, ok := subedPatchesMap.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to convert patched map to map[string]interface{}")
	}

	return typedMap, nil
}

func NewPatcher(strategicMergePatch apiextensions.JSON, jsonPatch string) patch.Patcher {
	if strategicMergePatch != nil {
		return patch.NewPatchStrategicMerge(strategicMergePatch)
	}
	if len(jsonPatch) > 0 {
		return patch.NewPatchesJSON6902(jsonPatch)
	}
	return nil
}
