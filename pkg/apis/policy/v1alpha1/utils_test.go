package v1alpha1

import (
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var defaultResourceDescriptionName = "defaultResourceDescription"
var defaultResourceDescription = ResourceDescription{
	Kinds: []string{"Deployment"},
	Name:  &defaultResourceDescriptionName,
	Selector: &metav1.LabelSelector{
		MatchLabels: map[string]string{"LabelForSelector": "defaultResourceDescription"},
	},
}

func Test_EmptyRule(t *testing.T) {
	emptyRule := Rule{
		Name:                "defaultRule",
		ResourceDescription: defaultResourceDescription,
	}
	err := emptyRule.Validate()
	assert.Assert(t, err != nil)
}

func Test_ResourceDescription(t *testing.T) {
	err := defaultResourceDescription.Validate()
	assert.NilError(t, err)
}

func Test_ResourceDescription_EmptyKind(t *testing.T) {
	resourceDescription := ResourceDescription{
		Name: &defaultResourceDescriptionName,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"LabelForSelector": "defaultResourceDescription"},
		},
	}
	err := resourceDescription.Validate()
	assert.Assert(t, err != nil)
}

func Test_ResourceDescription_EmptyNameAndSelector(t *testing.T) {
	resourceDescription := ResourceDescription{
		Kinds: []string{"Deployment"},
	}
	err := resourceDescription.Validate()
	assert.Assert(t, err != nil)
}

func Test_Patch_EmptyPath(t *testing.T) {
	patch := Patch{
		Operation: "add",
		Value:     "true",
	}
	err := patch.Validate()
	assert.Assert(t, err != nil)
}

func Test_Patch_EmptyValueWithAdd(t *testing.T) {
	patch := Patch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
	}
	err := patch.Validate()
	assert.Assert(t, err != nil)
}

func Test_Patch_UnsupportedOperation(t *testing.T) {
	patch := Patch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "overwrite",
		Value:     "true",
	}
	err := patch.Validate()
	assert.Assert(t, err != nil)
}

func Test_Generation_EmptyCopyFrom(t *testing.T) {
	generation := Generation{
		Kind: "ConfigMap",
		Name: "comfigmapGenerator",
	}
	err := generation.Validate()
	assert.Assert(t, err != nil)
}
