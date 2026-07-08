package policy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewCELMutateUR(t *testing.T) {
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-mpol"},
	}

	ur := newCELMutateUR(mpol)

	assert.NotNil(t, ur)
	assert.Equal(t, kyvernov2.CELMutate, ur.Spec.Type)
	assert.Equal(t, "my-mpol", ur.Spec.Policy)
	assert.Equal(t, kyvernov2.SchemeGroupVersion.String(), ur.TypeMeta.APIVersion)
	assert.Equal(t, "UpdateRequest", ur.TypeMeta.Kind)
	assert.Equal(t, "ur-", ur.GenerateName)
}

func TestNewCELMutateURFromNamespacedPolicy(t *testing.T) {
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-nmpol", Namespace: "my-ns"},
	}

	ur := newCELMutateURFromNamespacedPolicy(nmpol)

	assert.NotNil(t, ur)
	assert.Equal(t, kyvernov2.CELMutate, ur.Spec.Type)
	assert.Equal(t, "my-ns/my-nmpol", ur.Spec.Policy)
	assert.Equal(t, kyvernov2.SchemeGroupVersion.String(), ur.TypeMeta.APIVersion)
	assert.Equal(t, "UpdateRequest", ur.TypeMeta.Kind)
	assert.Equal(t, "ur-", ur.GenerateName)
}
