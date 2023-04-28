package mutate

import (
	"encoding/json"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

// type Response struct {
// 	Status  engineapi.RuleStatus
// 	Patches []jsonpatch.JsonPatchOperation
// 	Message string
// }

// func NewResponse(status engineapi.RuleStatus, patches []jsonpatch.JsonPatchOperation, msg string) *Response {
// 	return &Response{
// 		Status:  status,
// 		Patches: patches,
// 		Message: msg,
// 	}
// }

// func NewErrorResponse(msg string, err error) *Response {
// 	if err != nil {
// 		msg = fmt.Sprintf("%s: %v", msg, err)
// 	}
// 	return NewResponse(engineapi.RuleStatusError, nil, msg)
// }

func Mutate(logger logr.Logger, rule *kyvernov1.Rule, ctx context.Interface) (patch.Patcher, error) {
	updatedRule, err := variables.SubstituteAllInRule(logger, ctx, *rule)
	if err != nil {
		return nil, err
	}
	patcher := NewPatcher(updatedRule.Mutation.GetPatchStrategicMerge(), updatedRule.Mutation.PatchesJSON6902)
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
