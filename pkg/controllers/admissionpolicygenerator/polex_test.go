package admissionpolicygenerator

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TestEnqueueExceptionContinuesOnMissingPolicy verifies that enqueueException
// continues processing remaining exceptions even when one policy lookup fails.
// This is a regression test for a bug where the function returned early on
// the first error, skipping all subsequent exceptions.
func TestEnqueueExceptionContinuesOnMissingPolicy(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	policy1 := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "policy-1",
		},
	}
	policy2 := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "policy-2",
		},
	}

	indexer.Add(policy1)
	indexer.Add(policy2)

	lister := kyvernov1listers.NewClusterPolicyLister(indexer)

	c := &controller{
		cpolLister: lister,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
		),
	}

	policies, err := lister.List(labels.Everything())
	assert.NoError(t, err)
	assert.Len(t, policies, 2, "should have 2 policies in lister")

	// Create an exception that references three policies:
	// - policy-1: exists, should be enqueued
	// - policy-missing: does not exist, should be skipped
	// - policy-2: exists, should be enqueued
	exception := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-exception",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "policy-1", RuleNames: []string{"rule1"}},
				{PolicyName: "policy-missing", RuleNames: []string{"rule1"}},
				{PolicyName: "policy-2", RuleNames: []string{"rule1"}},
			},
		},
	}

	c.enqueueException(exception)

	assert.Equal(t, 2, c.queue.Len(),
		"both existing policies should be enqueued even when one is missing in between")

	enqueuedPolicies := make(map[string]bool)
	for c.queue.Len() > 0 {
		item, _ := c.queue.Get()
		enqueuedPolicies[item.(string)] = true
		c.queue.Done(item)
	}

	assert.True(t, enqueuedPolicies["ClusterPolicy/policy-1"], "policy-1 should be enqueued")
	assert.True(t, enqueuedPolicies["ClusterPolicy/policy-2"], "policy-2 should be enqueued despite policy-missing error")
}

// TestEnqueueCELExceptionContinuesOnMissingPolicy verifies that enqueueCELException
// continues processing remaining policy refs even when one policy lookup fails.
func TestEnqueueCELExceptionContinuesOnMissingPolicy(t *testing.T) {
	vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	mpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	vpol1 := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vpol-1",
		},
	}
	vpol2 := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vpol-2",
		},
	}
	mpol1 := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mpol-1",
		},
	}

	vpolIndexer.Add(vpol1)
	vpolIndexer.Add(vpol2)
	mpolIndexer.Add(mpol1)

	vpolLister := policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer)
	mpolLister := policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer)

	c := &controller{
		vpolLister: vpolLister,
		mpolLister: mpolLister,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
		),
	}

	vpolicies, err := vpolLister.List(labels.Everything())
	assert.NoError(t, err)
	assert.Len(t, vpolicies, 2, "should have 2 validating policies in lister")

	mpolicies, err := mpolLister.List(labels.Everything())
	assert.NoError(t, err)
	assert.Len(t, mpolicies, 1, "should have 1 mutating policy in lister")

	// Create a CEL exception that references multiple policies with one missing in between
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cel-exception",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "vpol-1", Kind: "ValidatingPolicy"},
				{Name: "vpol-missing", Kind: "ValidatingPolicy"},
				{Name: "vpol-2", Kind: "ValidatingPolicy"},
				{Name: "mpol-missing", Kind: "MutatingPolicy"},
				{Name: "mpol-1", Kind: "MutatingPolicy"},
			},
		},
	}

	c.enqueueCELException(exception)

	assert.Equal(t, 3, c.queue.Len(),
		"all existing policies should be enqueued even when some are missing")

	enqueuedPolicies := make(map[string]bool)
	for c.queue.Len() > 0 {
		item, _ := c.queue.Get()
		enqueuedPolicies[item.(string)] = true
		c.queue.Done(item)
	}

	assert.True(t, enqueuedPolicies["ValidatingPolicy/vpol-1"], "vpol-1 should be enqueued")
	assert.True(t, enqueuedPolicies["ValidatingPolicy/vpol-2"], "vpol-2 should be enqueued despite vpol-missing error")
	assert.True(t, enqueuedPolicies["MutatingPolicy/mpol-1"], "mpol-1 should be enqueued despite mpol-missing error")
}
