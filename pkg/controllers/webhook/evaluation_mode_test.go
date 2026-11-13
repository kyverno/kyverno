package webhook

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestGetValidatingPoliciesSkipsForeignMode(t *testing.T) {
	owned := &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "owned"}}
	envoy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "envoy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "Envoy"},
		},
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(owned))
	require.NoError(t, indexer.Add(envoy))

	c := &controller{vpolLister: policiesv1beta1listers.NewValidatingPolicyLister(indexer)}
	got, err := c.getValidatingPolicies()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "owned", got[0].GetName())
}
