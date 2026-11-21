package policy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestCastPolicy_LegacyKindsReturnNil(t *testing.T) {
	// castPolicy only recognizes CEL policy kinds; a legacy kyverno.io/v1 kind
	// (represented as unstructured, since the Go types are gone) must fall
	// through to the default case and return nil.
	tests := []struct {
		name string
		kind string
	}{
		{name: "ClusterPolicy", kind: "ClusterPolicy"},
		{name: "Policy", kind: "Policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legacy := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "kyverno.io/v1",
				"kind":       tt.kind,
				"metadata": map[string]interface{}{
					"name": "test-policy",
				},
			}}
			result := castPolicy(legacy)
			assert.Nil(t, result)
		})
	}
}
