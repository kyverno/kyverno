package v1

import (
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_ClusterPolicy_Name(t *testing.T) {
	subject := ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
			Namespace: "abcd",
		},
	}
	errs := subject.Validate(nil)
	assert.Assert(t, len(errs) == 1)
	assert.Equal(t, errs[0].Field, "name")
	assert.Equal(t, errs[0].Type, field.ErrorTypeTooLong)
	assert.Equal(t, errs[0].Detail, "must have at most 63 bytes")
	assert.Equal(t, errs[0].Error(), "name: Too long: must have at most 63 bytes")
}

func Test_ClusterPolicy_IsNamespaced(t *testing.T) {
	namespaced := Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
			Namespace: "abcd",
		},
	}
	notNamespaced := ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
		},
	}
	assert.Equal(t, namespaced.IsNamespaced(), true)
	assert.Equal(t, notNamespaced.IsNamespaced(), false)
}

func Test_ClusterPolicy_Autogen_All(t *testing.T) {
	subject := ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "policy",
			Annotations: map[string]string{
				kyverno.AnnotationAutogenControllers: "all",
			},
		},
	}
	errs := subject.Validate(nil)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Error(), "metadata.annotations: Forbidden: Autogen annotation does not support 'all' anymore, remove the annotation or set it to a valid value")
}

func Test_ClusterPolicy_HasAutoGenAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "Policy with AutoGen annotation (true)",
			annotations: map[string]string{kyverno.AnnotationAutogenControllers: "pod-policies.kyverno.io/autogen-controllers"},
			expected:    true,
		},
		{
			name:        "Policy with AutoGen annotation (false)",
			annotations: map[string]string{kyverno.AnnotationAutogenControllers: "none"},
			expected:    false,
		},
		{
			name:        "Policy without AutoGen annotation",
			annotations: map[string]string{},
			expected:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := &ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Annotations: tc.annotations}}
			result := policy.HasAutoGenAnnotation()
			if result != tc.expected {
				t.Errorf("Expected HasAutoGenAnnotation for policy %s to be %t, but got %t", tc.name, tc.expected, result)
			}
		})
	}
}
