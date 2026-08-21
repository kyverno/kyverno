package api

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewPolicyException_GetAPIVersionAndKind(t *testing.T) {
	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "skip-dev",
			Namespace: "default",
		},
	}

	exception := NewPolicyException(polex)

	assert.Equal(t, kyvernov2.GroupVersion.String(), exception.GetAPIVersion())
	assert.Equal(t, "PolicyException", exception.GetKind())
}

func TestNewCELPolicyException_GetAPIVersionAndKind(t *testing.T) {
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "skip-dev",
			Namespace: "default",
		},
	}

	exception := NewCELPolicyException(polex)

	assert.Equal(t, policiesv1beta1.GroupVersion.String(), exception.GetAPIVersion())
	assert.Equal(t, "PolicyException", exception.GetKind())
}
