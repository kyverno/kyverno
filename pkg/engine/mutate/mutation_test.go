package mutate

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/context"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	types "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"

	"github.com/kyverno/kyverno/pkg/logging"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyverno/kyverno/pkg/engine/utils"
)

// jsonPatch is used to build test patches
type jsonPatch struct {
	Path      string             `json:"path,omitempty" yaml:"path,omitempty"`
	Operation string             `json:"op,omitempty" yaml:"op,omitempty"`
	Value     apiextensions.JSON `json:"value,omitempty" yaml:"value,omitempty"`
}

const endpointsDocument string = `{
	"kind": "Endpoints",
	"apiVersion": "v1",
	"metadata": {
		"name": "my-endpoint-service",
		"labels": {
			"originalLabel": "isHere"
		}
	},
	"subsets": [
		{
			"addresses": [
				{
					"ip": "1.2.3.4"
				}
			],
			"ports": [
				{
					"port": 9376
				}
			]
		}
	]
}`

func applyPatches(rule *types.Rule, resource unstructured.Unstructured) (*response.RuleResponse, unstructured.Unstructured) {
	mutateResp := Mutate(rule, context.NewContext(), resource, logging.GlobalLogger())

	if mutateResp.Status != response.RuleStatusPass {
		return &response.RuleResponse{
			Type:    response.Mutation,
			Status:  mutateResp.Status,
			Message: mutateResp.Message,
		}, resource
	}

	return &response.RuleResponse{
		Type:    response.Mutation,
		Status:  response.RuleStatusPass,
		Patches: mutateResp.Patches,
	}, mutateResp.PatchedResource
}

func TestProcessPatches_EmptyPatches(t *testing.T) {
	var emptyRule = &types.Rule{Name: "emptyRule"}
	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}

	rr, _ := applyPatches(emptyRule, *resourceUnstructured)
	assert.Equal(t, rr.Status, response.RuleStatusError)
	assert.Assert(t, len(rr.Patches) == 0)
}

func makeAddIsMutatedLabelPatch() jsonPatch {
	return jsonPatch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     "true",
	}
}

func makeRuleWithPatch(t *testing.T, patch jsonPatch) *types.Rule {
	patches := []jsonPatch{patch}
	return makeRuleWithPatches(t, patches)
}

func makeRuleWithPatches(t *testing.T, patches []jsonPatch) *types.Rule {
	jsonPatches, err := json.Marshal(patches)
	if err != nil {
		t.Errorf("failed to marshal patch: %v", err)
	}

	mutation := types.Mutation{
		PatchesJSON6902: string(jsonPatches),
	}
	return &types.Rule{
		Mutation: mutation,
	}
}

func TestProcessPatches_EmptyDocument(t *testing.T) {
	rule := makeRuleWithPatch(t, makeAddIsMutatedLabelPatch())
	rr, _ := applyPatches(rule, unstructured.Unstructured{})
	assert.Equal(t, rr.Status, response.RuleStatusFail)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_AllEmpty(t *testing.T) {
	emptyRule := &types.Rule{}
	rr, _ := applyPatches(emptyRule, unstructured.Unstructured{})
	assert.Equal(t, rr.Status, response.RuleStatusError)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_AddPathDoesntExist(t *testing.T) {
	patch := makeAddIsMutatedLabelPatch()
	patch.Path = "/metadata/additional/is-mutated"
	rule := makeRuleWithPatch(t, patch)
	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := applyPatches(rule, *resourceUnstructured)
	assert.Equal(t, rr.Status, response.RuleStatusSkip)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_RemovePathDoesntExist(t *testing.T) {
	patch := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	rule := makeRuleWithPatch(t, patch)
	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := applyPatches(rule, *resourceUnstructured)
	assert.Equal(t, rr.Status, response.RuleStatusSkip)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_AddAndRemovePathsDontExist_EmptyResult(t *testing.T) {
	patch1 := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := jsonPatch{Path: "/spec/labels/label3", Operation: "add", Value: "label3Value"}
	rule := makeRuleWithPatches(t, []jsonPatch{patch1, patch2})
	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := applyPatches(rule, *resourceUnstructured)
	assert.Equal(t, rr.Status, response.RuleStatusPass)
	assert.Equal(t, len(rr.Patches), 1)
}

func TestProcessPatches_AddAndRemovePathsDontExist_ContinueOnError_NotEmptyResult(t *testing.T) {
	patch1 := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := jsonPatch{Path: "/spec/labels/label2", Operation: "remove", Value: "label2Value"}
	patch3 := jsonPatch{Path: "/metadata/labels/label3", Operation: "add", Value: "label3Value"}
	rule := makeRuleWithPatches(t, []jsonPatch{patch1, patch2, patch3})
	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}

	rr, _ := applyPatches(rule, *resourceUnstructured)
	assert.Equal(t, rr.Status, response.RuleStatusPass)
	assert.Assert(t, len(rr.Patches) != 0)
	assertEqStringAndData(t, `{"path":"/metadata/labels/label3","op":"add","value":"label3Value"}`, rr.Patches[0])
}

func TestProcessPatches_RemovePathDoesntExist_EmptyResult(t *testing.T) {
	patch := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	rule := makeRuleWithPatch(t, patch)
	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := applyPatches(rule, *resourceUnstructured)
	assert.Equal(t, rr.Status, response.RuleStatusSkip)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_RemovePathDoesntExist_NotEmptyResult(t *testing.T) {
	patch1 := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := jsonPatch{Path: "/metadata/labels/label2", Operation: "add", Value: "label2Value"}
	rule := makeRuleWithPatches(t, []jsonPatch{patch1, patch2})
	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := applyPatches(rule, *resourceUnstructured)
	assert.Equal(t, rr.Status, response.RuleStatusPass)
	assert.Assert(t, len(rr.Patches) == 1)
	assertEqStringAndData(t, `{"path":"/metadata/labels/label2","op":"add","value":"label2Value"}`, rr.Patches[0])
}

func assertEqStringAndData(t *testing.T, str string, data []byte) {
	var p1 jsonPatch
	json.Unmarshal([]byte(str), &p1)

	var p2 jsonPatch
	json.Unmarshal([]byte(data), &p2)

	assert.Equal(t, p1, p2)
}
