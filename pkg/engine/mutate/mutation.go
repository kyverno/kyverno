package mutate

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Response struct {
	Status          response.RuleStatus
	PatchedResource unstructured.Unstructured
	Patches         [][]byte
	Message         string
}

func NewErrorResponse(msg string, err error) *Response {
	return NewResponse(response.RuleStatusError, unstructured.Unstructured{}, nil, fmt.Sprintf("%s: %v", msg, err))
}

func NewResponse(status response.RuleStatus, resource unstructured.Unstructured, patches [][]byte, msg string) *Response {
	return &Response{
		Status:          status,
		PatchedResource: resource,
		Patches:         patches,
		Message:         msg,
	}
}

func Mutate(rule *kyvernov1.Rule, ctx context.Interface, resource unstructured.Unstructured, logger logr.Logger) *Response {
	updatedRule, err := variables.SubstituteAllInRule(logger, ctx, *rule)
	if err != nil {
		return NewErrorResponse("variable substitution failed", err)
	}

	m := updatedRule.Mutation
	patcher := NewPatcher(updatedRule.Name, m.GetPatchStrategicMerge(), m.PatchesJSON6902, resource, ctx, logger)
	if patcher == nil {
		return NewResponse(response.RuleStatusError, resource, nil, "empty mutate rule")
	}

	resp, patchedResource := patcher.Patch()
	if resp.Status != response.RuleStatusPass {
		return NewResponse(resp.Status, resource, nil, resp.Message)
	}

	if resp.Patches == nil {
		return NewResponse(response.RuleStatusSkip, resource, nil, "no patches applied")
	}

	if rule.IsMutateExisting() {
		if err := ctx.AddTargetResource(patchedResource.Object); err != nil {
			return NewErrorResponse("failed to update patched resource in the JSON context", err)
		}
	} else {
		if err := ctx.AddResource(patchedResource.Object); err != nil {
			return NewErrorResponse("failed to update patched resource in the JSON context", err)
		}
	}

	return NewResponse(response.RuleStatusPass, patchedResource, resp.Patches, resp.Message)
}

func ForEach(name string, foreach kyvernov1.ForEachMutation, ctx context.Interface, resource unstructured.Unstructured, logger logr.Logger) *Response {
	fe, err := substituteAllInForEach(foreach, ctx, logger)
	if err != nil {
		return NewErrorResponse("variable substitution failed", err)
	}

	patcher := NewPatcher(name, fe.GetPatchStrategicMerge(), fe.PatchesJSON6902, resource, ctx, logger)
	if patcher == nil {
		return NewResponse(response.RuleStatusError, unstructured.Unstructured{}, nil, "no patches found")
	}

	resp, patchedResource := patcher.Patch()
	if resp.Status != response.RuleStatusPass {
		return NewResponse(resp.Status, unstructured.Unstructured{}, nil, resp.Message)
	}

	if resp.Patches == nil {
		return NewResponse(response.RuleStatusSkip, unstructured.Unstructured{}, nil, "no patches applied")
	}

	if err := ctx.AddResource(patchedResource.Object); err != nil {
		return NewErrorResponse("failed to update patched resource in the JSON context", err)
	}

	return NewResponse(response.RuleStatusPass, patchedResource, resp.Patches, resp.Message)
}

func substituteAllInForEach(fe kyvernov1.ForEachMutation, ctx context.Interface, logger logr.Logger) (*kyvernov1.ForEachMutation, error) {
	jsonObj, err := datautils.ToMap(fe)
	if err != nil {
		return nil, err
	}

	data, err := variables.SubstituteAll(logger, ctx, jsonObj)
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var updatedForEach kyvernov1.ForEachMutation
	if err := json.Unmarshal(bytes, &updatedForEach); err != nil {
		return nil, err
	}

	return &updatedForEach, nil
}

func NewPatcher(name string, strategicMergePatch apiextensions.JSON, jsonPatch string, r unstructured.Unstructured, ctx context.Interface, logger logr.Logger) patch.Patcher {
	if strategicMergePatch != nil {
		return patch.NewPatchStrategicMerge(name, strategicMergePatch, r, ctx, logger)
	}

	if len(jsonPatch) > 0 {
		return patch.NewPatchesJSON6902(name, jsonPatch, r, logger)
	}

	return nil
}
