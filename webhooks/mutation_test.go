package webhooks_test

import (
	"testing"

	"github.com/nirmata/kube-policy/webhooks"

	//v1beta1 "k8s.io/api/admission/v1beta1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
)

func TestSerializePatches_Empty(t *testing.T) {
	var patches []types.PolicyPatch
	bytes, err := webhooks.SerializePatches(patches)
	assertEq(t, nil, err)
	assertEqStringAndData(t, "[\n\n]", bytes)
}

func TestSerializePatches_SingleValid(t *testing.T) {
	patch := types.PolicyPatch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     "true",
	}
	patches := []types.PolicyPatch{patch}
	bytes, err := webhooks.SerializePatches(patches)
	assertEq(t, nil, err)
	assertEqStringAndData(t, `[
{"path":"/metadata/labels/is-mutated","op":"add","value":"true"}
]`, bytes)
}

func TestSerializePatches_SingleInvalid(t *testing.T) {
	patch := types.PolicyPatch{
		Path:  "/metadata/labels/is-mutated",
		Value: "true",
	}
	patches := []types.PolicyPatch{patch}
	_, err := webhooks.SerializePatches(patches)
	assertNe(t, nil, err)
	patches[0].Path = ""
	patches[0].Operation = "delete"
	_, err = webhooks.SerializePatches(patches)
	assertNe(t, nil, err)
}
