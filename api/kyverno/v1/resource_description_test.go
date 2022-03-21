package v1

import (
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_ResourceDescription(t *testing.T) {
	testCases := []struct {
		name       string
		namespaced bool
		subject    ResourceDescription
		errors     []string
	}{{
		name:       "valid",
		namespaced: true,
		subject:    ResourceDescription{},
	}, {
		name:       "namespaces",
		namespaced: true,
		subject: ResourceDescription{
			Namespaces: []string{"abc"},
		},
		errors: []string{
			"dummy.namespaces: Forbidden: Filtering namespaces not allowed in namespaced policies",
		},
	}}

	path := field.NewPath("dummy")
	for _, testCase := range testCases {
		errs := testCase.subject.Validate(path, testCase.namespaced, nil)
		assert.Equal(t, len(errs), len(testCase.errors))
		for i, err := range errs {
			assert.Equal(t, err.Error(), testCase.errors[i])
		}
	}
}
