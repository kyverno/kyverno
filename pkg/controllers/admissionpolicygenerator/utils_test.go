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

	mapol, err := c.getMutatingAdmissionPolicy("test")
	assert.Error(t, err)
	assert.Nil(t, mapol)
	assert.Contains(t, err.Error(), "MutatingAdmissionPolicy lister is nil")

	mapbinding, err := c.getMutatingAdmissionPolicyBinding("test")
	assert.Error(t, err)
	assert.Nil(t, mapbinding)
	assert.Contains(t, err.Error(), "MutatingAdmissionPolicyBinding lister is nil")
}
