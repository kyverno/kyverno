package webhook

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// Admission-disabled MutatingPolicies must never be registered in the admission
// webhook, even when mutateExisting is enabled. With admission disabled, the
// policy opts out of the admission plane entirely; mutate-existing is driven by
// policy events and the periodic background scan only.
// See https://github.com/kyverno/kyverno/issues/16090.
func TestGetMutatingPoliciesExcludesAdmissionDisabled(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	assert.NoError(t, indexer.Add(&policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "admission-enabled"},
		Spec:       policiesv1beta1.MutatingPolicySpec{},
	}))
	assert.NoError(t, indexer.Add(&policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-existing-only"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				Admission:                   &policiesv1beta1.AdmissionConfiguration{Enabled: ptr.To(false)},
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: ptr.To(true)},
			},
		},
	}))

	c := &controller{mpolLister: policiesv1beta1listers.NewMutatingPolicyLister(indexer)}
	policies, err := c.getMutatingPolicies()

	assert.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, "admission-enabled", policies[0].GetName())
}

func TestGetNamespacedMutatingPoliciesExcludesAdmissionDisabled(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	assert.NoError(t, indexer.Add(&policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "admission-enabled", Namespace: "test-ns"},
		Spec:       policiesv1beta1.MutatingPolicySpec{},
	}))
	assert.NoError(t, indexer.Add(&policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-existing-only", Namespace: "test-ns"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				Admission:                   &policiesv1beta1.AdmissionConfiguration{Enabled: ptr.To(false)},
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: ptr.To(true)},
			},
		},
	}))

	c := &controller{nmpolLister: policiesv1beta1listers.NewNamespacedMutatingPolicyLister(indexer)}
	policies, err := c.getNamespacedMutatingPolicies()

	assert.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, "admission-enabled", policies[0].GetName())
	assert.Equal(t, "test-ns", policies[0].GetNamespace())
}
