package mutate

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Response struct {
	Status          response.RuleStatus
	PatchedResource unstructured.Unstructured
	Patches         [][]byte
	Message         string
}

func newErrorResponse(msg string, err error) *Response {
	return newResponse(response.RuleStatusError, unstructured.Unstructured{}, nil, fmt.Sprintf("%s: %v", msg, err))
}

func newResponse(status response.RuleStatus, resource unstructured.Unstructured, patches [][]byte, msg string) *Response {
	return &Response{
		Status:          status,
		PatchedResource: resource,
		Patches:         patches,
		Message:         msg,
	}
}

func Mutate(rule *kyverno.Rule, ctx context.Interface, resource unstructured.Unstructured, logger logr.Logger) *Response {
	updatedRule, err := variables.SubstituteAllInRule(logger, ctx, *rule)
	if err != nil {
		return newErrorResponse("variable substitution failed", err)
	}

	m := updatedRule.Mutation
	patcher := NewPatcher(updatedRule.Name, m.GetPatchStrategicMerge(), m.PatchesJSON6902, resource, ctx, logger)
	if patcher == nil {
		return newResponse(response.RuleStatusError, resource, nil, "empty mutate rule")
	}

	resp, patchedResource := patcher.Patch()
	if resp.Status != response.RuleStatusPass {
		return newResponse(resp.Status, resource, nil, resp.Message)
	}

	if resp.Patches == nil {
		return newResponse(response.RuleStatusSkip, resource, nil, "no patches applied")
	}

	if err := ctx.AddResource(patchedResource.Object); err != nil {
		return newErrorResponse("failed to update patched resource in the JSON context", err)
	}

	return newResponse(response.RuleStatusPass, patchedResource, resp.Patches, resp.Message)
}

func ForEach(name string, foreach kyverno.ForEachMutation, ctx context.Interface, resource unstructured.Unstructured, logger logr.Logger) *Response {
	fe, err := substituteAllInForEach(foreach, ctx, logger)
	if err != nil {
		return newErrorResponse("variable substitution failed", err)
	}

	patcher := NewPatcher(name, fe.GetPatchStrategicMerge(), fe.PatchesJSON6902, resource, ctx, logger)
	if patcher == nil {
		return newResponse(response.RuleStatusError, unstructured.Unstructured{}, nil, "no patches found")
	}

	resp, patchedResource := patcher.Patch()
	if resp.Status != response.RuleStatusPass {
		return newResponse(resp.Status, unstructured.Unstructured{}, nil, resp.Message)
	}

	if resp.Patches == nil {
		return newResponse(response.RuleStatusSkip, unstructured.Unstructured{}, nil, "no patches applied")
	}

	if err := ctx.AddResource(patchedResource.Object); err != nil {
		return newErrorResponse("failed to update patched resource in the JSON context", err)
	}

	return newResponse(response.RuleStatusPass, patchedResource, resp.Patches, resp.Message)
}

func substituteAllInForEach(fe kyverno.ForEachMutation, ctx context.Interface, logger logr.Logger) (*kyverno.ForEachMutation, error) {
	jsonObj, err := utils.ToMap(fe)
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

	var updatedForEach kyverno.ForEachMutation
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
