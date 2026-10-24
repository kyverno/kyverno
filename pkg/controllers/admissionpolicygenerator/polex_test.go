package admissionpolicygenerator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
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

// delayRecorder records AddAfter without starting timers; immediate operations use the embedded queue.
type delayRecorder struct {
	workqueue.TypedRateLimitingInterface[any]
	delays map[any]time.Duration
}

func (r *delayRecorder) AddAfter(item any, delay time.Duration) {
	r.delays[item] = delay
}

func TestEnqueueCELExceptionEnqueuesPoliciesImmediately(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *metav1.Time
	}{
		{name: "expires in the future", expiresAt: &metav1.Time{Time: time.Now().Add(time.Hour)}},
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
			assert.Empty(t, queue.delays, "expiry scheduling belongs to generation")
		})
	}
}

func TestRequeueAtExpiry(t *testing.T) {
	for _, kind := range []string{"ValidatingPolicy", "MutatingPolicy"} {
		t.Run(kind, func(t *testing.T) {
			for _, tt := range []struct {
				name       string
				offsets    []time.Duration
				wantOffset time.Duration
			}{
				{name: "no exceptions"},
				{name: "no expiry", offsets: []time.Duration{0}},
				{name: "all expired", offsets: []time.Duration{-time.Hour}},
				{name: "future expiry", offsets: []time.Duration{time.Hour}, wantOffset: time.Hour},
				{name: "earliest expiry", offsets: []time.Duration{2 * time.Hour, time.Hour, 0, -time.Hour}, wantOffset: time.Hour},
			} {
				t.Run(tt.name, func(t *testing.T) {
					now := time.Now()
					var exceptions []*policiesv1beta1.PolicyException
					for i, offset := range tt.offsets {
						exception := &policiesv1beta1.PolicyException{
							ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("exception-%d", i), Namespace: "default"},
							Spec: policiesv1beta1.PolicyExceptionSpec{
								PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "policy", Kind: kind}},
							},
						}
						if offset != 0 {
							exception.Spec.ExpiresAt = &metav1.Time{Time: now.Add(offset)}
						}
						exceptions = append(exceptions, exception)
					}
					queue := &delayRecorder{delays: map[any]time.Duration{}}
					c := &controller{
						celpolexLister:     newCELPolexLister(t, exceptions...),
						polexEnabled:       true,
						exceptionNamespace: "*",
						queue:              queue,
					}
					filtered, err := c.getCELExceptions("policy", kind)
					require.NoError(t, err)
					before := time.Now()
					c.requeueAtExpiry("policy", kind, filtered)
					after := time.Now()
					if tt.wantOffset == 0 {
						assert.Empty(t, queue.delays)
						return
					}
					require.Len(t, queue.delays, 1)
					delay, ok := queue.delays[kind+"/policy"]
					require.True(t, ok)
					expiry := now.Add(tt.wantOffset)
					assert.GreaterOrEqual(t, delay, expiry.Sub(after))
					assert.LessOrEqual(t, delay, expiry.Sub(before))
				})
			}
		})
	}
}

func TestRequeueAtExpiryExpiredDuringGeneration(t *testing.T) {
	queue := &delayRecorder{delays: map[any]time.Duration{}}
	c := &controller{queue: queue}
	c.requeueAtExpiry("policy", "ValidatingPolicy", []policiesv1beta1.PolicyException{{
		Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: &metav1.Time{Time: time.Now().Add(-time.Second)}},
	}})
	require.Contains(t, queue.delays, "ValidatingPolicy/policy")
	assert.LessOrEqual(t, queue.delays["ValidatingPolicy/policy"], time.Duration(0), "an exception expiring after listing must trigger immediate regeneration")
}

func TestDeleteCELExceptionEnqueuesReferencedPolicies(t *testing.T) {
	vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	mpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	exceptionIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, vpolIndexer.Add(&policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "vpol-1"}}))
	require.NoError(t, mpolIndexer.Add(&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "mpol-1"}}))
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exception", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "vpol-1", Kind: "ValidatingPolicy"},
				{Name: "mpol-1", Kind: "MutatingPolicy"},
			},
		},
	}
	require.NoError(t, exceptionIndexer.Add(exception))
	queue := &delayRecorder{
		TypedRateLimitingInterface: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
		),
		delays: map[any]time.Duration{},
	}
	defer queue.ShutDown()
	c := &controller{
		vpolLister:         policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer),
		mpolLister:         policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer),
		celpolexLister:     policiesv1beta1listers.NewPolicyExceptionLister(exceptionIndexer),
		queue:              queue,
		polexEnabled:       true,
		exceptionNamespace: "*",
	}

	// Informers remove the exception from their cache before delivering its delete event.
	require.NoError(t, exceptionIndexer.Delete(exception))
	c.deleteCELException(exception)

	require.Equal(t, 2, queue.Len(), "both referenced policies must be enqueued immediately")
	assert.Empty(t, queue.delays, "a non-expiring exception must not schedule delayed work")
	for _, ref := range exception.Spec.PolicyRefs {
		item, shutdown := queue.Get()
		require.False(t, shutdown)
		assert.Equal(t, ref.Kind+"/"+ref.Name, item)
		queue.Done(item)

		exceptions, err := c.getCELExceptions(ref.Name, ref.Kind)
		require.NoError(t, err)
		assert.Empty(t, exceptions, "generation must no longer see the deleted exception")
	}
}

func TestReconcileRearmsExceptionExpiry(t *testing.T) {
	t.Setenv("FLAG_GENERATE_VALIDATING_ADMISSION_POLICY", "true")
	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.Kind = "ValidatingPolicy"
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	c, kubeClient, _ := newVAPTestController(t, vpol)
	vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, vpolIndexer.Add(vpol))
	c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer)
	exceptionIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c.celpolexLister = policiesv1beta1listers.NewPolicyExceptionLister(exceptionIndexer)
	c.polexEnabled, c.exceptionNamespace = true, "*"
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c.vapLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer)
	c.vapbindingLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer)
	queue := &delayRecorder{
		TypedRateLimitingInterface: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]()),
		delays:                     map[any]time.Duration{},
	}
	defer queue.ShutDown()
	c.queue = queue
	newException := func(name string, expiry time.Time) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: vpol.Name, Kind: vpol.Kind}},
				MatchConditions: []admissionregistrationv1.MatchCondition{{Name: name, Expression: "true"}},
				ExpiresAt:       &metav1.Time{Time: expiry},
			},
		}
	}
	first := newException("first", time.Now().Add(time.Hour))
	second := newException("second", time.Now().Add(2*time.Hour))
	require.NoError(t, exceptionIndexer.Add(first))
	require.NoError(t, exceptionIndexer.Add(second))

	reconcile := func(wantExpiry *metav1.Time, wantConditions int) {
		t.Helper()
		clear(queue.delays)
		before := time.Now()
		require.NoError(t, c.reconcile(context.Background(), logr.Discard(), vpol.Kind+"/"+vpol.Name, "", vpol.Name))
		after := time.Now()
		if wantExpiry == nil {
			assert.Empty(t, queue.delays)
		} else {
			require.Len(t, queue.delays, 1)
			delay, ok := queue.delays[vpol.Kind+"/"+vpol.Name]
			require.True(t, ok)
			assert.GreaterOrEqual(t, delay, wantExpiry.Sub(after))
			assert.LessOrEqual(t, delay, wantExpiry.Sub(before))
		}
		vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), "vpol-"+vpol.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, vap.Spec.MatchConditions, wantConditions)
		vap.ResourceVersion = "1"
		require.NoError(t, vapIndexer.Update(vap))
		binding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.Background(), "vpol-"+vpol.Name+"-binding", metav1.GetOptions{})
		require.NoError(t, err)
		binding.ResourceVersion = "1"
		require.NoError(t, bindingIndexer.Update(binding))
	}
	reconcile(first.Spec.ExpiresAt, 3)
	// Simulate the first deadline without waiting for wall-clock time to pass.
	first.Spec.ExpiresAt = &metav1.Time{Time: time.Now().Add(-time.Hour)}
	require.NoError(t, exceptionIndexer.Update(first))
	reconcile(second.Spec.ExpiresAt, 2)
	second.Spec.ExpiresAt = &metav1.Time{Time: time.Now().Add(3 * time.Hour)}
	require.NoError(t, exceptionIndexer.Update(second))
	reconcile(second.Spec.ExpiresAt, 2)
	second.Spec.ExpiresAt = first.Spec.ExpiresAt
	require.NoError(t, exceptionIndexer.Update(second))
	reconcile(nil, 1)
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
