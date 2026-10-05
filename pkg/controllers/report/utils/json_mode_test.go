package utils

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// JSON mode MutatingPolicies have no cluster resources to report on; the report
// controllers must never pick them up even though background defaults to enabled.
func TestFetchMutatingPoliciesExcludesJSONMode(t *testing.T) {
	jsonSpec := policiesv1beta1.MutatingPolicySpec{
		EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	assert.NoError(t, indexer.Add(&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes"}}))
	assert.NoError(t, indexer.Add(&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "json"}, Spec: jsonSpec}))
	policies, err := FetchMutatingPolicies(policiesv1beta1listers.NewMutatingPolicyLister(indexer))
	assert.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, "kubernetes", policies[0].GetName())

	nsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	assert.NoError(t, nsIndexer.Add(&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "ns"}}))
	assert.NoError(t, nsIndexer.Add(&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "json", Namespace: "ns"}, Spec: jsonSpec}))
	nsPolicies, err := FetchNamespacedMutatingPolicies(policiesv1beta1listers.NewNamespacedMutatingPolicyLister(nsIndexer), "ns")
	assert.NoError(t, err)
	assert.Len(t, nsPolicies, 1)
	assert.Equal(t, "kubernetes", nsPolicies[0].GetName())
}
