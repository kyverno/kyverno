package engine

import (
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	types "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

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

func TestProcessPatches_EmptyPatches(t *testing.T) {
	var emptyRule = types.Rule{}
	resourceUnstructured, err := ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := processPatches(emptyRule, *resourceUnstructured)
	assert.Check(t, rr.Success)
	assert.Assert(t, len(rr.Patches) == 0)
}

func makeAddIsMutatedLabelPatch() types.Patch {
	return types.Patch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     "true",
	}
}

func makeRuleWithPatch(patch types.Patch) types.Rule {
	patches := []types.Patch{patch}
	return makeRuleWithPatches(patches)
}

func makeRuleWithPatches(patches []types.Patch) types.Rule {
	mutation := types.Mutation{
		Patches: patches,
	}
	return types.Rule{
		Mutation: mutation,
	}
}

func TestProcessPatches_EmptyDocument(t *testing.T) {
	rule := makeRuleWithPatch(makeAddIsMutatedLabelPatch())
	rr, _ := processPatches(rule, unstructured.Unstructured{})
	assert.Assert(t, !rr.Success)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_AllEmpty(t *testing.T) {
	emptyRule := types.Rule{}
	rr, _ := processPatches(emptyRule, unstructured.Unstructured{})
	assert.Check(t, !rr.Success)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_AddPathDoesntExist(t *testing.T) {
	patch := makeAddIsMutatedLabelPatch()
	patch.Path = "/metadata/additional/is-mutated"
	rule := makeRuleWithPatch(patch)
	resourceUnstructured, err := ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := processPatches(rule, *resourceUnstructured)
	assert.Check(t, !rr.Success)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_RemovePathDoesntExist(t *testing.T) {
	patch := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	rule := makeRuleWithPatch(patch)
	resourceUnstructured, err := ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := processPatches(rule, *resourceUnstructured)
	assert.Check(t, rr.Success)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_AddAndRemovePathsDontExist_EmptyResult(t *testing.T) {
	patch1 := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := types.Patch{Path: "/spec/labels/label3", Operation: "add", Value: "label3Value"}
	rule := makeRuleWithPatches([]types.Patch{patch1, patch2})
	resourceUnstructured, err := ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := processPatches(rule, *resourceUnstructured)
	assert.Check(t, !rr.Success)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_AddAndRemovePathsDontExist_ContinueOnError_NotEmptyResult(t *testing.T) {
	patch1 := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := types.Patch{Path: "/spec/labels/label2", Operation: "remove", Value: "label2Value"}
	patch3 := types.Patch{Path: "/metadata/labels/label3", Operation: "add", Value: "label3Value"}
	rule := makeRuleWithPatches([]types.Patch{patch1, patch2, patch3})
	resourceUnstructured, err := ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := processPatches(rule, *resourceUnstructured)
	assert.Check(t, rr.Success)
	assert.Assert(t, len(rr.Patches) != 0)
	assertEqStringAndData(t, `{"path":"/metadata/labels/label3","op":"add","value":"label3Value"}`, rr.Patches[0])
}

func TestProcessPatches_RemovePathDoesntExist_EmptyResult(t *testing.T) {
	patch := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	rule := makeRuleWithPatch(patch)
	resourceUnstructured, err := ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := processPatches(rule, *resourceUnstructured)
	assert.Check(t, rr.Success)
	assert.Assert(t, len(rr.Patches) == 0)
}

func TestProcessPatches_RemovePathDoesntExist_NotEmptyResult(t *testing.T) {
	patch1 := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := types.Patch{Path: "/metadata/labels/label2", Operation: "add", Value: "label2Value"}
	rule := makeRuleWithPatches([]types.Patch{patch1, patch2})
	resourceUnstructured, err := ConvertToUnstructured([]byte(endpointsDocument))
	if err != nil {
		t.Error(err)
	}
	rr, _ := processPatches(rule, *resourceUnstructured)
	assert.Check(t, rr.Success)
	assert.Assert(t, len(rr.Patches) == 1)
	assertEqStringAndData(t, `{"path":"/metadata/labels/label2","op":"add","value":"label2Value"}`, rr.Patches[0])
}

func assertEqDataImpl(t *testing.T, expected, actual []byte, formatModifier string) {
	if len(expected) != len(actual) {
		t.Errorf("len(expected) != len(actual): %d != %d\n1:"+formatModifier+"\n2:"+formatModifier, len(expected), len(actual), expected, actual)
		return
	}

	for idx, val := range actual {
		if val != expected[idx] {
			t.Errorf("Slices not equal at index %d:\n1:"+formatModifier+"\n2:"+formatModifier, idx, expected, actual)
		}
	}
}

func assertEqData(t *testing.T, expected, actual []byte) {
	assertEqDataImpl(t, expected, actual, "%x")
}

func assertEqStringAndData(t *testing.T, str string, data []byte) {
	assertEqDataImpl(t, []byte(str), data, "%s")
}
