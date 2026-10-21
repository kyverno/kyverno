package admissionpolicygenerator

import (
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

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

// delayRecorder wraps a real queue and records every AddAfter call, so a test can check what
// the controller scheduled without waiting for the delay to pass.
type delayRecorder struct {
	workqueue.TypedRateLimitingInterface[any]
	delays map[any]time.Duration
}

func (r *delayRecorder) AddAfter(item any, delay time.Duration) {
	r.delays[item] = delay
	r.TypedRateLimitingInterface.AddAfter(item, delay)
}

// TestEnqueueCELExceptionRequeuesPoliciesAtExpiry verifies that the policies an exception
// references are reconciled again when it expires. getCELExceptions drops expired exceptions,
// but only when generation runs; nothing else enqueues the policy at the deadline, so without
// this requeue a generated VAP/MAP would keep the exception's negated match conditions after it
// expired. Exceptions without expiresAt, or already expired, schedule nothing extra.
func TestEnqueueCELExceptionRequeuesPoliciesAtExpiry(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *metav1.Time
		wantDelay bool
	}{
		{name: "expires in the future", expiresAt: &metav1.Time{Time: time.Now().Add(time.Hour)}, wantDelay: true},
		{name: "no expiry", expiresAt: nil},
		{name: "already expired", expiresAt: &metav1.Time{Time: time.Now().Add(-time.Hour)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			mpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			assert.NoError(t, vpolIndexer.Add(&policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "vpol-1"}}))
			assert.NoError(t, mpolIndexer.Add(&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "mpol-1"}}))

			queue := &delayRecorder{
				TypedRateLimitingInterface: workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[any](),
					workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
				),
				delays: map[any]time.Duration{},
			}
			defer queue.ShutDown()
			c := &controller{
				vpolLister: policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer),
				mpolLister: policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer),
				queue:      queue,
			}

			c.addCELException(&policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Name: "exception", Namespace: "default"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{
						{Name: "vpol-1", Kind: "ValidatingPolicy"},
						{Name: "mpol-1", Kind: "MutatingPolicy"},
					},
					ExpiresAt: tt.expiresAt,
				},
			})

			assert.Equal(t, 2, queue.Len(), "both policies are enqueued immediately in every case")
			if !tt.wantDelay {
				assert.Empty(t, queue.delays, "nothing may be scheduled for later")
				return
			}
			for _, key := range []string{"ValidatingPolicy/vpol-1", "MutatingPolicy/mpol-1"} {
				delay, ok := queue.delays[key]
				if assert.True(t, ok, "%s must be requeued at expiry", key) {
					assert.InDelta(t, time.Hour, delay, float64(time.Minute), "%s must be requeued when the exception expires", key)
				}
			}
		})
	}
}

// TestGetCELExceptionsIgnoresExpired is a regression guard for the expiry gate, which
// already worked correctly before the exception-gating change. Its fixtures deliberately
// share the same kind and name, differing only in ExpiresAt, so it keeps testing expiry
// rather than the other gates.
func TestGetCELExceptionsIgnoresExpired(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	active := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "active-exception",
			Namespace: "default",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: "vpol-1",
				Kind: "ValidatingPolicy",
			}},
			ExpiresAt: &metav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}
	expired := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "expired-exception",
			Namespace: "default",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: "vpol-1",
				Kind: "ValidatingPolicy",
			}},
			ExpiresAt: &metav1.Time{
				Time: time.Now().Add(-time.Hour),
			},
		},
	}
	assert.NoError(t, indexer.Add(active))
	assert.NoError(t, indexer.Add(expired))

	lister := policiesv1beta1listers.NewPolicyExceptionLister(indexer)
	c := &controller{
		celpolexLister:     lister,
		polexEnabled:       true,
		exceptionNamespace: "*",
	}
	got, err := c.getCELExceptions("vpol-1", "ValidatingPolicy")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "active-exception", got[0].GetName())
}

// names are unique only within a kind, so a MutatingPolicy exception must not surface for a
// same-named ValidatingPolicy: it would otherwise be baked into that policy's generated VAP.
func TestGetCELExceptionsFiltersByKind(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	newException := func(name, kind string) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "shared-name", Kind: kind}},
			},
		}
	}
	assert.NoError(t, indexer.Add(newException("vpol-exception", "ValidatingPolicy")))
	assert.NoError(t, indexer.Add(newException("mpol-exception", "MutatingPolicy")))

	c := &controller{
		celpolexLister:     policiesv1beta1listers.NewPolicyExceptionLister(indexer),
		polexEnabled:       true,
		exceptionNamespace: "*",
	}

	got, err := c.getCELExceptions("shared-name", "ValidatingPolicy")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "vpol-exception", got[0].GetName())

	got, err = c.getCELExceptions("shared-name", "MutatingPolicy")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "mpol-exception", got[0].GetName())
}

// --- PolicyException gate regression tests ---
//
// These cover the runtime gates getCELExceptions must apply, matching what the CEL
// admission path already enforces: the --enablePolicyException flag and
// --exceptionNamespace scoping.

func newCELPolexLister(t *testing.T, polexs ...*policiesv1beta1.PolicyException) policiesv1beta1listers.PolicyExceptionLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, p := range polexs {
		assert.NoError(t, indexer.Add(p))
	}
	return policiesv1beta1listers.NewPolicyExceptionLister(indexer)
}

func TestGetCELExceptions_DisabledFlag(t *testing.T) {
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "cel-polex-1", Namespace: "ns-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "ValidatingPolicy"}},
		},
	}
	lister := newCELPolexLister(t, polex)

	disabled := &controller{celpolexLister: lister, polexEnabled: false, exceptionNamespace: "*"}
	got, err := disabled.getCELExceptions("vpol-1", "ValidatingPolicy")
	assert.NoError(t, err)
	assert.Nil(t, got)

	// Positive control: the same exception, with the flag enabled, IS returned.
	enabled := &controller{celpolexLister: lister, polexEnabled: true, exceptionNamespace: "*"}
	got, err = enabled.getCELExceptions("vpol-1", "ValidatingPolicy")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}

// TestGetCELExceptions_NamespaceScope also documents that the CEL path reads both "" and
// "*" as cluster-wide, matching engine.NewPolicyExceptionLister.
func TestGetCELExceptions_NamespaceScope(t *testing.T) {
	polexA := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "cel-polex-a", Namespace: "ns-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "ValidatingPolicy"}},
		},
	}
	polexB := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "cel-polex-b", Namespace: "ns-b"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "ValidatingPolicy"}},
		},
	}
	lister := newCELPolexLister(t, polexA, polexB)

	scoped := &controller{celpolexLister: lister, polexEnabled: true, exceptionNamespace: "ns-a"}
	got, err := scoped.getCELExceptions("vpol-1", "ValidatingPolicy")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "cel-polex-a", got[0].GetName())

	for _, ns := range []string{"", "*"} {
		clusterWide := &controller{celpolexLister: lister, polexEnabled: true, exceptionNamespace: ns}
		got, err = clusterWide.getCELExceptions("vpol-1", "ValidatingPolicy")
		assert.NoError(t, err)
		assert.Len(t, got, 2, "exceptionNamespace %q must be cluster-wide on the CEL path", ns)
	}
}
