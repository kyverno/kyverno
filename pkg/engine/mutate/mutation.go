package mutate

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/mattbaird/jsonpatch"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Response struct {
	Status          engineapi.RuleStatus
	PatchedResource unstructured.Unstructured
	Patches         []jsonpatch.JsonPatchOperation
	Message         string
}

func NewResponse(status engineapi.RuleStatus, resource unstructured.Unstructured, patches []jsonpatch.JsonPatchOperation, msg string) *Response {
	return &Response{
		Status:          status,
		PatchedResource: resource,
		Patches:         patches,
		Message:         msg,
	}
}

func NewErrorResponse(msg string, err error) *Response {
	if err != nil {
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	return NewResponse(engineapi.RuleStatusError, unstructured.Unstructured{}, nil, msg)
}

func Mutate(rule *kyvernov1.Rule, ctx context.Interface, resource unstructured.Unstructured, logger logr.Logger) *Response {
	updatedRule, err := variables.SubstituteAllInRule(logger, ctx, *rule)
	if err != nil {
		return NewErrorResponse("variable substitution failed", err)
	}
	m := updatedRule.Mutation
	patcher := NewPatcher(m.GetPatchStrategicMerge(), m.PatchesJSON6902)
	if patcher == nil {
		return NewErrorResponse("empty mutate rule", nil)
	}
	resourceBytes, err := resource.MarshalJSON()
	if err != nil {
		return NewErrorResponse("failed to marshal resource", err)
	}
	resourceBytes, patches, err := patcher.Patch(logger, resourceBytes)
	if err != nil {
		return NewErrorResponse("failed to patch resource", err)
	}
	if len(patches) == 0 {
		return NewResponse(engineapi.RuleStatusSkip, resource, nil, "no patches applied")
	}
	if err := resource.UnmarshalJSON(resourceBytes); err != nil {
		return NewErrorResponse("failed to unmarshal patched resource", err)
	}
	if rule.IsMutateExisting() {
		if err := ctx.SetTargetResource(resource.Object); err != nil {
			return NewErrorResponse("failed to update patched resource in the JSON context", err)
		}
	} else {
		if err := ctx.AddResource(resource.Object); err != nil {
			return NewErrorResponse("failed to update patched resource in the JSON context", err)
		}
	}
	return NewResponse(engineapi.RuleStatusPass, resource, patches, "resource patched")
}

func ForEach(name string, foreach kyvernov1.ForEachMutation, policyContext engineapi.PolicyContext, resource unstructured.Unstructured, element interface{}, logger logr.Logger) *Response {
	ctx := policyContext.JSONContext()
	fe, err := substituteAllInForEach(foreach, ctx, logger)
	if err != nil {
		return NewErrorResponse("variable substitution failed", err)
	}
	patcher := NewPatcher(fe.GetPatchStrategicMerge(), fe.PatchesJSON6902)
	if patcher == nil {
		return NewErrorResponse("empty mutate rule", nil)
	}
	resourceBytes, err := resource.MarshalJSON()
	if err != nil {
		return NewErrorResponse("failed to marshal resource", err)
	}
	resourceBytes, patches, err := patcher.Patch(logger, resourceBytes)
	if err != nil {
		return NewErrorResponse("failed to patch resource", err)
	}
	if len(patches) == 0 {
		return NewResponse(engineapi.RuleStatusSkip, resource, nil, "no patches applied")
	}
	if err := resource.UnmarshalJSON(resourceBytes); err != nil {
		return NewErrorResponse("failed to unmarshal patched resource", err)
	} else if err := ctx.AddResource(resource.Object); err != nil {
		return NewErrorResponse("failed to update patched resource in the JSON context", err)
	} else {
		return NewResponse(engineapi.RuleStatusPass, resource, patches, "resource patched")
	}
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

func NewPatcher(strategicMergePatch apiextensions.JSON, jsonPatch string) patch.Patcher {
	if strategicMergePatch != nil {
		return patch.NewPatchStrategicMerge(strategicMergePatch)
	}
	if len(jsonPatch) > 0 {
		return patch.NewPatchesJSON6902(jsonPatch)
	}
	return nil
}
