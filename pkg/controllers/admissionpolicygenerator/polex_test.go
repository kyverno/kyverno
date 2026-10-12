package admissionpolicygenerator

import (
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
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

// TestGetCELExceptionsIgnoresExpired is a regression guard for the expiry gate, which
// already worked correctly before the exception-gating change. Its fixtures deliberately
// share the same kind and name, differing only in ExpiresAt, so it keeps testing expiry
// rather than the kind-matching gate added alongside it.
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

// --- PolicyException gate regression tests ---
//
// These cover the runtime gates that getExceptions (kyvernov2, ClusterPolicy branch) and
// getCELExceptions (v1beta1, ValidatingPolicy/MutatingPolicy branches) must now apply,
// matching what the runtime admission path already enforces: enable flag, exception
// namespace scoping, expiry (CEL only), and policy-reference kind (CEL only).

func newV2PolexLister(t *testing.T, polexs ...*kyvernov2.PolicyException) kyvernov2listers.PolicyExceptionLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, p := range polexs {
		assert.NoError(t, indexer.Add(p))
	}
	return kyvernov2listers.NewPolicyExceptionLister(indexer)
}

func newCELPolexLister(t *testing.T, polexs ...*policiesv1beta1.PolicyException) policiesv1beta1listers.PolicyExceptionLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, p := range polexs {
		assert.NoError(t, indexer.Add(p))
	}
	return policiesv1beta1listers.NewPolicyExceptionLister(indexer)
}

// TestGetExceptions_DisabledFlag is gate 1 (enable flag) for the v2/ClusterPolicy path.
func TestGetExceptions_DisabledFlag(t *testing.T) {
	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "polex-1", Namespace: "ns-a"},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{PolicyName: "cpol-1", RuleNames: []string{"rule-1"}}},
		},
	}
	lister := newV2PolexLister(t, polex)

	disabled := &controller{polexLister: lister, polexEnabled: false, exceptionNamespace: "*"}
	got, err := disabled.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Nil(t, got)

	// Positive control: the same exception, with the flag enabled, IS returned.
	enabled := &controller{polexLister: lister, polexEnabled: true, exceptionNamespace: "*"}
	got, err = enabled.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}

// TestGetCELExceptions_DisabledFlag is gate 1 (enable flag) for the CEL path.
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

	// Positive control.
	enabled := &controller{celpolexLister: lister, polexEnabled: true, exceptionNamespace: "*"}
	got, err = enabled.getCELExceptions("vpol-1", "ValidatingPolicy")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}

// TestGetExceptions_NamespaceScope is gate 2 (namespace scoping) for the v2 path. It also
// documents that v2 reads an empty exceptionNamespace as "exceptions disabled", not
// cluster-wide -- the opposite of the CEL path's reading. Each selector mirrors its own
// runtime path, and those two genuinely disagree here; see getExceptions' own comment.
func TestGetExceptions_NamespaceScope(t *testing.T) {
	polexA := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "polex-a", Namespace: "ns-a"},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{PolicyName: "cpol-1", RuleNames: []string{"rule-1"}}},
		},
	}
	polexB := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "polex-b", Namespace: "ns-b"},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{PolicyName: "cpol-1", RuleNames: []string{"rule-1"}}},
		},
	}
	lister := newV2PolexLister(t, polexA, polexB)

	scoped := &controller{polexLister: lister, polexEnabled: true, exceptionNamespace: "ns-a"}
	got, err := scoped.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "polex-a", got[0].GetName())

	empty := &controller{polexLister: lister, polexEnabled: true, exceptionNamespace: ""}
	got, err = empty.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Nil(t, got, "empty exceptionNamespace must mean 'no exceptions' on the v2 path")

	clusterWide := &controller{polexLister: lister, polexEnabled: true, exceptionNamespace: "*"}
	got, err = clusterWide.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Len(t, got, 2)
}

// TestGetCELExceptions_NamespaceScope is gate 2 (namespace scoping) for the CEL path. It
// documents that CEL reads both "" and "*" as cluster-wide, matching
// engine.NewPolicyExceptionLister -- the regression guard for the intentional "" divergence
// from the v2 path above.
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

// TestGetCELExceptions_KindGate is gate 4 (policy reference kind) for the CEL path, the
// gap that used to let a MutatingPolicy exception silently apply to a ValidatingPolicy of
// the same name. The v2 path has no equivalent -- kyvernov2.Exception has no Kind field
// (kyvernov2.PolicyException has no kind field on its references, so this gate is CEL-only).
func TestGetCELExceptions_KindGate(t *testing.T) {
	wrongKind := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "wrong-kind", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "MutatingPolicy"}},
		},
	}
	crossName := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "cross-name", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "other-pol", Kind: "ValidatingPolicy"}},
		},
	}
	correct := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "correct", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "ValidatingPolicy"}},
		},
	}
	lister := newCELPolexLister(t, wrongKind, crossName, correct)
	c := &controller{celpolexLister: lister, polexEnabled: true, exceptionNamespace: "*"}

	got, err := c.getCELExceptions("vpol-1", "ValidatingPolicy")
	assert.NoError(t, err)
	assert.Len(t, got, 1, "only the correctly-kinded, correctly-named exception should be returned")
	assert.Equal(t, "correct", got[0].GetName())
}

// TestGetExceptions_GatesFeedCanGenerateVAP is the denial-of-enforcement regression guard:
// an exception with Conditions set (which would make admissionpolicy.CanGenerateVAP delete
// the live VAP, see pkg/admissionpolicy/kyvernopolicy_checker.go) must never reach that
// check when the gates would have excluded it at runtime.
func TestGetExceptions_GatesFeedCanGenerateVAP(t *testing.T) {
	dangerous := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "dangerous", Namespace: "ns-b"},
		Spec: kyvernov2.PolicyExceptionSpec{
			Conditions: &kyvernov2.AnyAllConditions{},
			Exceptions: []kyvernov2.Exception{{PolicyName: "cpol-1", RuleNames: []string{"rule-1"}}},
		},
	}
	lister := newV2PolexLister(t, dangerous)

	// Disabled flag excludes it.
	disabled := &controller{polexLister: lister, polexEnabled: false, exceptionNamespace: "*"}
	got, err := disabled.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Empty(t, got, "an exception the runtime would not honor must not reach CanGenerateVAP")

	// Wrong namespace excludes it too.
	wrongNamespace := &controller{polexLister: lister, polexEnabled: true, exceptionNamespace: "ns-a"}
	got, err = wrongNamespace.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Empty(t, got)

	// Positive control: correctly gated, it IS returned (and would still hit CanGenerateVAP,
	// which is unchanged wiring -- out of scope here).
	correct := &controller{polexLister: lister, polexEnabled: true, exceptionNamespace: "ns-b"}
	got, err = correct.getExceptions("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}
