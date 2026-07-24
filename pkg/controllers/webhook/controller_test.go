package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// newReadyMutateClusterPolicy builds a ClusterPolicy with a single standard
// mutate rule (so it matches the mutating webhook type) whose status starts Ready.
func newReadyMutateClusterPolicy(name string) *kyvernov1.ClusterPolicy {
	p := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{{
				Name: "add-team-label",
				Mutation: &kyvernov1.Mutation{
					RawPatchStrategicMerge: &apiextv1.JSON{Raw: []byte(`{"metadata":{"labels":{"team":"platform"}}}`)},
				},
			}},
		},
	}
	p.Status.SetReady(true, "Ready")
	return p
}

// newStatusTestController wires the minimal controller fields updatePolicyStatuses
// and watchdogCheck touch: the kyverno client (status writes), the policy listers
// (getAllPolicies), the lease lister (watchdog health), the recorded policy state
// and the autoUpdateWebhooks toggle.
func newStatusTestController(client versioned.Interface, cpols []*kyvernov1.ClusterPolicy, lease *coordinationv1.Lease, state map[string]sets.Set[string]) *controller {
	cpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, p := range cpols {
		_ = cpolIndexer.Add(p)
	}
	leaseIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if lease != nil {
		_ = leaseIndexer.Add(lease)
	}
	return &controller{
		kyvernoClient:      client,
		cpolLister:         kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:          kyvernov1listers.NewPolicyLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
		leaseLister:        coordinationv1listers.NewLeaseLister(leaseIndexer),
		policyState:        state,
		autoUpdateWebhooks: true,
	}
}

// fakeRuntime is a runtimeutils.Runtime stub whose only meaningful answer is
// IsRollingUpdate() == false, so reconcile takes the normal (non rolling update)
// branch where the watchdog guard lives.
type fakeRuntime struct{}

func (fakeRuntime) IsDebug() bool                { return false }
func (fakeRuntime) IsReady(context.Context) bool { return true }
func (fakeRuntime) IsLive(context.Context) bool  { return true }
func (fakeRuntime) IsRollingUpdate() bool        { return false }
func (fakeRuntime) IsGoingDown() bool            { return false }

// recordingQueue wraps a real queue and records every AddAfter the controller
// makes, so a test can assert the reconcile requeued rather than rebuilt.
type recordingQueue struct {
	workqueue.TypedRateLimitingInterface[any]
	added []any
}

func (q *recordingQueue) AddAfter(item any, delay time.Duration) {
	q.added = append(q.added, item)
	q.TypedRateLimitingInterface.AddAfter(item, delay)
}

// TestReconcile_RequeuesInsteadOfPublishingEmptyWebhooksWhenWatchdogUnhealthy is
// the regression for guard B. When webhook health is not confirmed, reconcile must
// requeue the resource webhooks instead of rebuilding them: an unhealthy rebuild
// produces an empty webhook set, and publishing that would wipe the persisted
// configuration and drop enforcement for every policy at once. The controller is
// wired with NO webhook client and NO secret lister on purpose: if the guard ever
// regressed and the rebuild ran, it would reach those nil dependencies and panic,
// so this test fails loudly on regression in addition to asserting the requeue.
func TestReconcile_RequeuesInsteadOfPublishingEmptyWebhooksWhenWatchdogUnhealthy(t *testing.T) {
	ctx := context.Background()

	baseQ := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]())
	defer baseQ.ShutDown()
	q := &recordingQueue{TypedRateLimitingInterface: baseQ}

	// No lease -> watchdogCheck() is false (health unknown). No webhook clients or
	// secret lister: the guard must keep the rebuild path from ever touching them.
	c := &controller{
		runtime:     fakeRuntime{},
		queue:       q,
		leaseLister: coordinationv1listers.NewLeaseLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
		policyState: map[string]sets.Set[string]{
			config.MutatingWebhookConfigurationName:   sets.New[string](),
			config.ValidatingWebhookConfigurationName: sets.New[string](),
		},
		autoUpdateWebhooks: true,
	}
	require.False(t, c.watchdogCheck(), "precondition: no lease -> watchdog unhealthy")

	for _, name := range []string{config.MutatingWebhookConfigurationName, config.ValidatingWebhookConfigurationName} {
		require.NoError(t, c.reconcile(ctx, logr.Discard(), name, "", name),
			"reconcile must not error (or panic on a nil webhook client) while unhealthy: %s", name)
	}

	// Each reconcile requeues both resource webhooks, so both config names are present.
	assert.Contains(t, q.added, config.MutatingWebhookConfigurationName,
		"unhealthy reconcile should requeue the mutating resource webhook, not rebuild it")
	assert.Contains(t, q.added, config.ValidatingWebhookConfigurationName,
		"unhealthy reconcile should requeue the validating resource webhook, not rebuild it")
}

// TestUpdatePolicyStatuses_DoesNotDowngradeReadyPolicyWhenWatchdogUnhealthy is the
// regression for #11560 / #16281. On a fresh admission-controller pod (startup,
// leader change, or a cluster resumed after an outage) the recorded policy state is
// empty and the webhook watchdog has not yet confirmed health. updatePolicyStatuses
// must NOT flip an already-Ready policy to NotReady in that window: doing so evicts
// the policy from the admission policy cache, and the mutate handler then answers
// "allowed, no changes" so a failurePolicy: Fail mutate rule is silently skipped.
func TestUpdatePolicyStatuses_DoesNotDowngradeReadyPolicyWhenWatchdogUnhealthy(t *testing.T) {
	ctx := context.Background()
	policy := newReadyMutateClusterPolicy("add-team-label")
	require.True(t, policy.IsReady(), "precondition: policy starts Ready")

	client := versionedfake.NewSimpleClientset(policy.DeepCopy())
	// No lease -> watchdogCheck() is false (health unknown); empty policyState is the
	// freshly-constructed state before any healthy reconcile has run.
	state := map[string]sets.Set[string]{
		config.MutatingWebhookConfigurationName: sets.New[string](),
	}
	c := newStatusTestController(client, []*kyvernov1.ClusterPolicy{policy.DeepCopy()}, nil, state)
	require.False(t, c.watchdogCheck(), "precondition: no lease -> watchdog unhealthy")

	require.NoError(t, c.updatePolicyStatuses(ctx, config.MutatingWebhookConfigurationName))

	got, err := client.KyvernoV1().ClusterPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, got.IsReady(),
		"a Ready policy must not be flipped to NotReady while webhook health is unknown (that opens a fail-open admission window)")
}

// TestUpdatePolicyStatuses_DowngradesUnconfiguredPolicyWhenWatchdogHealthy is the
// control: once health is confirmed, a policy that is genuinely absent from the
// recorded webhook state is still correctly marked NotReady. This proves the
// watchdog guard is scoped to the unhealthy window and does not disable the normal
// readiness bookkeeping.
func TestUpdatePolicyStatuses_DowngradesUnconfiguredPolicyWhenWatchdogHealthy(t *testing.T) {
	ctx := context.Background()
	policy := newReadyMutateClusterPolicy("add-team-label")

	client := versionedfake.NewSimpleClientset(policy.DeepCopy())
	state := map[string]sets.Set[string]{
		config.MutatingWebhookConfigurationName: sets.New[string](),
	}
	freshLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "kyverno-health",
			Namespace:   config.KyvernoNamespace(),
			Annotations: map[string]string{AnnotationLastRequestTime: time.Now().Format(time.RFC3339)},
		},
	}
	c := newStatusTestController(client, []*kyvernov1.ClusterPolicy{policy.DeepCopy()}, freshLease, state)
	require.True(t, c.watchdogCheck(), "precondition: fresh lease -> watchdog healthy")

	require.NoError(t, c.updatePolicyStatuses(ctx, config.MutatingWebhookConfigurationName))

	got, err := client.KyvernoV1().ClusterPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, got.IsReady(),
		"when health is confirmed and the policy is not configured in any webhook, it is correctly marked NotReady")
}
