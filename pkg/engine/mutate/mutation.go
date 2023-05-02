package mutate

import (
	"encoding/json"
	"errors"

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

func Mutate(logger logr.Logger, rule *kyvernov1.Rule, ctx context.Interface) (patch.Patcher, error) {
	updatedRule, err := variables.SubstituteAllInRule(logger, ctx, *rule)
	if err != nil {
		return nil, err
	}
	patcher := NewPatcher(updatedRule.Mutation.GetPatchStrategicMerge(), updatedRule.Mutation.PatchesJSON6902)
	if patcher == nil {
		return nil, errors.New("failed to create patcher")
	}
	return patcher, nil
}

func ForEach(logger logr.Logger, foreach kyvernov1.ForEachMutation, policyContext engineapi.PolicyContext) (patch.Patcher, error) {
	ctx := policyContext.JSONContext()
	fe, err := substituteAllInForEach(foreach, ctx, logger)
	if err != nil {
		return nil, err
	}
	patcher := NewPatcher(fe.GetPatchStrategicMerge(), fe.PatchesJSON6902)
	return patcher, nil
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

func ApplyPatchers(logger logr.Logger, resource unstructured.Unstructured, rule kyvernov1.Rule, patchers ...patch.Patcher) (unstructured.Unstructured, *engineapi.RuleResponse) {
	if len(patchers) == 0 {
		return resource, engineapi.RuleSkip(rule.Name, engineapi.Mutation, "no patches")
	}
	// apply patchers
	resourceBytes, err := resource.MarshalJSON()
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return resource, engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to marshal resource", err)
	}
	var allPatches []jsonpatch.JsonPatchOperation
	for _, patcher := range patchers {
		patchedBytes, patches, err := patcher.Patch(logger, resourceBytes)
		if err != nil {
			logger.Error(err, "failed to patch resource")
			return resource, engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to patch resource", err)
		}
		resourceBytes = patchedBytes
		allPatches = append(allPatches, patches...)
	}
	if len(allPatches) == 0 {
		return resource, engineapi.RuleSkip(rule.Name, engineapi.Mutation, "no patches")
	}
	err = resource.UnmarshalJSON(resourceBytes)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		return resource, engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to unmarshal resource", err)
	}
	return resource, engineapi.RulePass(rule.Name, engineapi.Mutation, "TODO").WithPatches(patch.ConvertPatches(allPatches...)...)
}
