package v1

import (
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		name:       "selector",
		namespaced: true,
		subject: ResourceDescription{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.type": "prod",
				},
			},
		},
	}, {
		name:       "bad-selector",
		namespaced: true,
		subject: ResourceDescription{
			Kinds:    []string{"Deployment"},
			Selector: &metav1.LabelSelector{},
		},
		errors: []string{
			`dummy.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string(nil), MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: The requirements are not specified in selector`,
		},
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
