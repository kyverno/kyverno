package webhook

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// JSON mode MutatingPolicies are evaluated against standalone documents and
// must never contribute rules or resource constraints to the admission webhook,
// even though their admission toggle defaults to enabled.
func TestGetMutatingPoliciesExcludesJSONMode(t *testing.T) {
	jsonSpec := policiesv1beta1.MutatingPolicySpec{
		EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
	}
	k8sSpec := policiesv1beta1.MutatingPolicySpec{
		EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeKubernetes},
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	assert.NoError(t, indexer.Add(&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes"}, Spec: k8sSpec}))
	assert.NoError(t, indexer.Add(&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "json"}, Spec: jsonSpec}))
	c := &controller{mpolLister: policiesv1beta1listers.NewMutatingPolicyLister(indexer)}
	policies, err := c.getMutatingPolicies()
	assert.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, "kubernetes", policies[0].GetName())

	nsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	assert.NoError(t, nsIndexer.Add(&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "ns"}, Spec: k8sSpec}))
	assert.NoError(t, nsIndexer.Add(&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "json", Namespace: "ns"}, Spec: jsonSpec}))
	c = &controller{nmpolLister: policiesv1beta1listers.NewNamespacedMutatingPolicyLister(nsIndexer)}
	nsPolicies, err := c.getNamespacedMutatingPolicies()
	assert.NoError(t, err)
	assert.Len(t, nsPolicies, 1)
	assert.Equal(t, "kubernetes", nsPolicies[0].GetName())
}
