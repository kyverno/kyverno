package v2beta1

import (
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Policy_Name(t *testing.T) {
	subject := Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
		},
	}
	_, errs := subject.Validate(nil)
	assert.Assert(t, len(errs) == 1)
	assert.Equal(t, errs[0].Field, "name")
	assert.Equal(t, errs[0].Type, field.ErrorTypeTooLong)
	assert.Equal(t, errs[0].Detail, "may not be more than 63 bytes")
	assert.Equal(t, errs[0].Error(), "name: Too long: may not be more than 63 bytes")
}

func Test_Policy_IsNamespaced(t *testing.T) {
	namespaced := Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
			Namespace: "abcd",
		},
	}
	notNamespaced := Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "this-is-a-way-too-long-policy-name-that-should-trigger-an-error-when-calling-the-policy-validation-method",
		},
	}
	assert.Equal(t, namespaced.IsNamespaced(), true)
	assert.Equal(t, notNamespaced.IsNamespaced(), true)
}

func Test_Policy_Autogen_All(t *testing.T) {
	subject := Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy",
			Namespace: "abcd",
			Annotations: map[string]string{
				kyverno.AnnotationAutogenControllers: "all",
			},
		},
	}
	_, errs := subject.Validate(nil)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Error(), "metadata.annotations: Forbidden: Autogen annotation does not support 'all' anymore, remove the annotation or set it to a valid value")
}
