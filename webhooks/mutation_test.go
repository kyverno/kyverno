package webhooks_test

import (
	"gotest.tools/assert"
	"testing"

	"github.com/nirmata/kube-policy/webhooks"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
)

func TestSerializePatches_Empty(t *testing.T) {
	var patches []types.PolicyPatch
	bytes, err := webhooks.SerializePatches(patches)
	assert.Assert(t, nil == err)
	assert.Assert(t, 0 == len(bytes))
}

func TestSerializePatches_SingleStringValid(t *testing.T) {
	patch := types.PolicyPatch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     "true",
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.Assert(t, nil == err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/is-mutated","op":"add","value":"true"}
]`, bytes)
}

func TestSerializePatches_SingleStringInvalid(t *testing.T) {
	patch := types.PolicyPatch{
		Path:  "/metadata/labels/is-mutated",
		Value: "true",
	}
	patches := []types.PolicyPatch{patch}
	_, err := webhooks.SerializePatches(patches)
	assert.Assert(t, nil != err)
	patches[0].Path = ""
	patches[0].Operation = "delete"
	_, err = webhooks.SerializePatches(patches)
	assert.Assert(t, nil != err)
}

func TestSerializePatches_MultipleStringsValid(t *testing.T) {
	patch1 := types.PolicyPatch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     "true",
	}
	patch2 := types.PolicyPatch{
		Path:      "/metadata/labels/newLabel",
		Operation: "add",
		Value:     "newValue",
	}
	patches := []types.PolicyPatch{patch1, patch2}
	bytes, err := webhooks.SerializePatches(patches)
	assert.Assert(t, nil == err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/is-mutated","op":"add","value":"true"},
{"path":"/metadata/labels/newLabel","op":"add","value":"newValue"}
]`, bytes)
}

func TestSerializePatches_SingleIntegerValid(t *testing.T) {
	const ordinaryInt int = 42
	patch := types.PolicyPatch{
		Path:      "/metadata/labels/int",
		Operation: "add",
		Value:     ordinaryInt,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/int","op":"add","value":"42"}
]`, bytes)
}

func TestSerializePatches_SingleIntegerBigValid(t *testing.T) {
	const bigInt uint64 = 100500100500
	patch := types.PolicyPatch{
		Path:      "/metadata/labels/big-int",
		Operation: "add",
		Value:     bigInt,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/big-int","op":"add","value":"100500100500"}
]`, bytes)
}

func TestSerializePatches_SingleFloatValid(t *testing.T) {
	const ordinaryFloat float32 = 2.71828
	patch := types.PolicyPatch{
		Path:      "/metadata/labels/float",
		Operation: "add",
		Value:     ordinaryFloat,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/float","op":"add","value":"2.71828"}
]`, bytes)
}

func TestSerializePatches_SingleFloatBigValid(t *testing.T) {
	const bigFloat float64 = 3.1415926535
	patch := types.PolicyPatch{
		Path:      "/metadata/labels/big-float",
		Operation: "add",
		Value:     bigFloat,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/big-float","op":"add","value":"3.1415926535"}
]`, bytes)
}

func TestSerializePatches_MultipleBoolValid(t *testing.T) {
	patch1 := types.PolicyPatch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     true,
	}
	patch2 := types.PolicyPatch{
		Path:      "/metadata/labels/is-unreal",
		Operation: "add",
		Value:     false,
	}
	patches := []types.PolicyPatch{patch1, patch2}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/is-mutated","op":"add","value":"true"},
{"path":"/metadata/labels/is-unreal","op":"add","value":"false"}
]`, bytes)
}

func TestSerializePatches_MultitypeMap(t *testing.T) {
	labelsMap := make(map[string]interface{})
	labelsMap["label1"] = "value1"
	labelsMap["label42"] = 42
	nestedMap := make(map[string]interface{})
	nestedMap["other label"] = "other value"
	nestedMap["nested"] = true
	labelsMap["additional"] = nestedMap

	patch := types.PolicyPatch{
		Path:      "/metadata/labels",
		Operation: "add",
		Value:     labelsMap,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels","op":"add","value":{"additional":{"nested":"true","other label":"other value"},"label1":"value1","label42":"42"}}
]`, bytes)
}
