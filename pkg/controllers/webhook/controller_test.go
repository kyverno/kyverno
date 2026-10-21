package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

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
		runtime:            fakeRuntime{},
		queue:              q,
		leaseLister:        coordinationv1listers.NewLeaseLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
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

// TestDefaultResourceWebhookConfigurations_NoLegacyCatchAll is a 1.20 regression
// guard: the static (autoUpdateWebhooks=false) default resource webhook
// configurations must never register a webhook targeting the removed legacy
// "/validate" or "/mutate" catch-all routes. Those routes no longer exist on the
// server mux, so a webhook pointed at them would make every create/update/delete
// of every resource fail (FailurePolicy: Fail) or silently no-op (Ignore).
func TestDefaultResourceWebhookConfigurations_NoLegacyCatchAll(t *testing.T) {
	c := &controller{
		defaultTimeout:    10,
		servicePort:       443,
		clusterroleLister: rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}
	cfg := config.NewDefaultConfiguration(false)

	mwc, err := c.buildDefaultResourceMutatingWebhookConfiguration(context.Background(), cfg, nil)
	require.NoError(t, err)
	for _, w := range mwc.Webhooks {
		require.NotNil(t, w.ClientConfig.Service)
		assert.NotEqual(t, config.MutatingWebhookServicePath, *w.ClientConfig.Service.Path,
			"default mutating webhook configuration must not target the legacy /mutate catch-all: %s", w.Name)
	}

	vwc, err := c.buildDefaultResourceValidatingWebhookConfiguration(context.Background(), cfg, nil)
	require.NoError(t, err)
	for _, w := range vwc.Webhooks {
		require.NotNil(t, w.ClientConfig.Service)
		assert.NotEqual(t, config.ValidatingWebhookServicePath, *w.ClientConfig.Service.Path,
			"default validating webhook configuration must not target the legacy /validate catch-all: %s", w.Name)
	}
}
