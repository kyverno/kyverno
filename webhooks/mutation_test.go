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
		Path:      "/spec/replicas",
		Operation: "add",
		Value:     ordinaryInt,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/spec/replicas","op":"add","value":42}
]`, bytes)
}

func TestSerializePatches_SingleIntegerBigValid(t *testing.T) {
	const bigInt uint64 = 100500100500
	patch := types.PolicyPatch{
		Path:      "/spec/somethingHuge",
		Operation: "add",
		Value:     bigInt,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/spec/somethingHuge","op":"add","value":100500100500}
]`, bytes)
}

func TestSerializePatches_SingleFloatValid(t *testing.T) {
	const ordinaryFloat float32 = 2.71828
	patch := types.PolicyPatch{
		Path:      "/spec/consts/e",
		Operation: "add",
		Value:     ordinaryFloat,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/spec/consts/e","op":"add","value":2.71828}
]`, bytes)
}

func TestSerializePatches_SingleFloatBigValid(t *testing.T) {
	const bigFloat float64 = 3.1415926535
	patch := types.PolicyPatch{
		Path:      "/spec/consts/pi",
		Operation: "add",
		Value:     bigFloat,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/spec/consts/pi","op":"add","value":3.1415926535}
]`, bytes)
}

func TestSerializePatches_MultipleBoolValid(t *testing.T) {
	patch1 := types.PolicyPatch{
		Path:      "/status/is-mutated",
		Operation: "add",
		Value:     true,
	}
	patch2 := types.PolicyPatch{
		Path:      "/status/is-unreal",
		Operation: "add",
		Value:     false,
	}
	patches := []types.PolicyPatch{patch1, patch2}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/status/is-mutated","op":"add","value":true},
{"path":"/status/is-unreal","op":"add","value":false}
]`, bytes)
}

func TestSerializePatches_MultitypeMap(t *testing.T) {
	valuesMap := make(map[string]interface{})
	valuesMap["some_string"] = "value1"
	valuesMap["number"] = 42
	nestedMap := make(map[string]interface{})
	nestedMap["label"] = "other value"
	nestedMap["nested"] = true
	valuesMap["additional"] = nestedMap

	patch := types.PolicyPatch{
		Path:      "/spec/values",
		Operation: "add",
		Value:     valuesMap,
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assert.NilError(t, err)
	assertEqStringAndData(t, `[
{"path":"/spec/values","op":"add","value":{"additional":{"label":"other value","nested":true},"number":42,"some_string":"value1"}}
]`, bytes)
}
