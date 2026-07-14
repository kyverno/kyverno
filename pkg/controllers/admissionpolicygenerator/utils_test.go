package admissionpolicygenerator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NilListers(t *testing.T) {
	c := &controller{}

	vap, err := c.getValidatingAdmissionPolicy("test")
	assert.Error(t, err)
	assert.Nil(t, vap)
	assert.Contains(t, err.Error(), "ValidatingAdmissionPolicy lister is nil")

	vapbinding, err := c.getValidatingAdmissionPolicyBinding("test")
	assert.Error(t, err)
	assert.Nil(t, vapbinding)
	assert.Contains(t, err.Error(), "ValidatingAdmissionPolicyBinding lister is nil")

	// Test v1alpha1 getters
	mapol, err := c.getMutatingAdmissionPolicy("test")
	assert.Error(t, err)
	assert.Nil(t, mapol)
	assert.Contains(t, err.Error(), "MutatingAdmissionPolicy v1alpha1 lister is nil")

	mapbinding, err := c.getMutatingAdmissionPolicyBinding("test")
	assert.Error(t, err)
	assert.Nil(t, mapbinding)
	assert.Contains(t, err.Error(), "MutatingAdmissionPolicyBinding v1alpha1 lister is nil")

	// Test v1beta1 getters
	mapolBeta, err := c.getMutatingAdmissionPolicyBeta("test")
	assert.Error(t, err)
	assert.Nil(t, mapolBeta)
	assert.Contains(t, err.Error(), "MutatingAdmissionPolicy v1beta1 lister is nil")

	mapbindingBeta, err := c.getMutatingAdmissionPolicyBindingBeta("test")
	assert.Error(t, err)
	assert.Nil(t, mapbindingBeta)
	assert.Contains(t, err.Error(), "MutatingAdmissionPolicyBinding v1beta1 lister is nil")
}
