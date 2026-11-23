package api

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenericPolicy_NamespacedGeneratingPolicy(t *testing.T) {
	ngpol := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ngpol",
			Namespace: "test-ns",
		},
	}

	p := NewNamespacedGeneratingPolicy(ngpol)
	assert.Equal(t, "test-ngpol", p.GetName())
	assert.Equal(t, "test-ns", p.GetNamespace())
	assert.Equal(t, "NamespacedGeneratingPolicy", p.GetKind())
	assert.Equal(t, policiesv1beta1.GroupVersion.String(), p.GetAPIVersion())
	assert.True(t, p.IsNamespaced())
	assert.NotNil(t, p.AsNamespacedGeneratingPolicy())
	assert.NotNil(t, p.AsGeneratingPolicyLike())
	assert.Nil(t, p.AsGeneratingPolicy())
}

func TestGenericPolicy_GeneratingPolicy(t *testing.T) {
	gpol := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gpol",
		},
	}

	p := NewGeneratingPolicy(gpol)
	assert.Equal(t, "test-gpol", p.GetName())
	assert.Equal(t, "", p.GetNamespace())
	assert.Equal(t, "GeneratingPolicy", p.GetKind())
	assert.Equal(t, policiesv1beta1.GroupVersion.String(), p.GetAPIVersion())
	assert.False(t, p.IsNamespaced())
	assert.NotNil(t, p.AsGeneratingPolicy())
	assert.NotNil(t, p.AsGeneratingPolicyLike())
	assert.Nil(t, p.AsNamespacedGeneratingPolicy())
}
