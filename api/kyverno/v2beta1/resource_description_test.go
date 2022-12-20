package v2beta1

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
		name:       "name-names",
		namespaced: true,
		subject: ResourceDescription{
			Names: []string{"bar", "baz"},
		},
		errors: []string{
			`dummy: Invalid value: v2beta1.ResourceDescription{Kinds:[]string(nil), Names:[]string{"bar", "baz"}, Namespaces:[]string(nil), Annotations:map[string]string(nil), Selector:(*v1.LabelSelector)(nil), NamespaceSelector:(*v1.LabelSelector)(nil)}: Both name and names can not be specified together`,
		},
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
