package policy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCastPolicy_MutatingPolicy(t *testing.T) {
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mpol"},
	}
	result := castPolicy(mpol)
	assert.NotNil(t, result)
	assert.NotNil(t, result.AsMutatingPolicy())
	assert.Equal(t, "test-mpol", result.AsMutatingPolicy().GetName())
	assert.IsType(t, engineapi.NewMutatingPolicy(mpol), result)
}

func TestCastPolicy_NamespacedMutatingPolicy(t *testing.T) {
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol", Namespace: "test-ns"},
	}
	result := castPolicy(nmpol)
	assert.NotNil(t, result)
	assert.NotNil(t, result.AsNamespacedMutatingPolicy())
	assert.Equal(t, "test-nmpol", result.AsNamespacedMutatingPolicy().GetName())
	assert.Equal(t, "test-ns", result.AsNamespacedMutatingPolicy().GetNamespace())
	assert.IsType(t, engineapi.NewNamespacedMutatingPolicy(nmpol), result)
}

// TestCastPolicy_LegacyKindsReturnNil pins that legacy kyvernov1 policy kinds
// are no longer recognized by castPolicy now that pkg/policy only dispatches
// CEL policies.
func TestCastPolicy_LegacyKindsReturnNil(t *testing.T) {
	cpol := &kyverno.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-cpol"}}
	assert.Nil(t, castPolicy(cpol))

	pol := &kyverno.Policy{ObjectMeta: metav1.ObjectMeta{Name: "test-pol", Namespace: "test-ns"}}
	assert.Nil(t, castPolicy(pol))
}
