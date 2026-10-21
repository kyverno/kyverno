package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
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

// legacyShapedWebhookPaths returns the 1.19 catch-all and fine-grained path shapes a
// persisted webhook configuration could still carry under the given base, matching what
// registerLegacyCompatRoutes (pkg/webhooks/server.go) serves for backward compat.
func legacyShapedWebhookPaths(base string) []string {
	return []string{
		base,
		base + "/ignore",
		base + "/fail",
		base + "/ignore" + config.FineGrainedWebhookPath + "/some-policy",
	}
}

func webhookWithPath(name, path string) admissionregistrationv1.ValidatingWebhook {
	return admissionregistrationv1.ValidatingWebhook{
		Name:         name,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Path: &path}},
	}
}

func mutatingWebhookWithPath(name, path string) admissionregistrationv1.MutatingWebhook {
	return admissionregistrationv1.MutatingWebhook{
		Name:         name,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Path: &path}},
	}
}

// TestReconcileResourceWebhookConfigurations_PrunesLegacyEntries is the regression for
// step 4: a webhook configuration persisted by 1.19 (or managed externally with
// autoUpdateWebhooks=false) may still carry the removed legacy "/validate" and "/mutate"
// entries. Since the server answers those paths allow-only (step 1a) rather than 404,
// nothing breaks immediately, but the stale entries should still be pruned once the
// reconciler gets a chance to run, so 1.21 can retire the compat routes. Matching is by
// ClientConfig.Service.Path, not webhook name, since operators own names in this mode.
func TestReconcileResourceWebhookConfigurations_PrunesLegacyEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var validatingWebhooks []admissionregistrationv1.ValidatingWebhook
	for _, path := range legacyShapedWebhookPaths(config.ValidatingWebhookServicePath) {
		validatingWebhooks = append(validatingWebhooks, webhookWithPath("legacy-"+path, path))
	}
	validatingWebhooks = append(validatingWebhooks, admissionregistrationv1.ValidatingWebhook{Name: config.GenerationLabelProtectionWebhookName})
	observedVWC := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: config.ValidatingWebhookConfigurationName},
		Webhooks:   validatingWebhooks,
	}

	var mutatingWebhooks []admissionregistrationv1.MutatingWebhook
	for _, path := range legacyShapedWebhookPaths(config.MutatingWebhookServicePath) {
		mutatingWebhooks = append(mutatingWebhooks, mutatingWebhookWithPath("legacy-"+path, path))
	}
	observedMWC := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: config.MutatingWebhookConfigurationName},
		Webhooks:   mutatingWebhooks,
	}

	client := kubefake.NewClientset(observedVWC.DeepCopy(), observedMWC.DeepCopy())
	vwcs := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, vwcs.Add(observedVWC))
	mwcs := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, mwcs.Add(observedMWC))
	secrets := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, secrets.Add(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: config.KyvernoNamespace()}, Data: map[string][]byte{corev1.TLSCertKey: []byte("ca")}}))

	c := &controller{
		defaultTimeout: 10, servicePort: 443, caSecretName: "ca", autoUpdateWebhooks: false,
		configuration:     config.NewDefaultConfiguration(false),
		vwcClient:         client.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		vwcLister:         admissionregistrationv1listers.NewValidatingWebhookConfigurationLister(vwcs),
		mwcClient:         client.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		mwcLister:         admissionregistrationv1listers.NewMutatingWebhookConfigurationLister(mwcs),
		secretLister:      corev1listers.NewSecretLister(secrets),
		clusterroleLister: rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}

	require.NoError(t, c.reconcileResourceValidatingWebhookConfiguration(ctx))
	updatedVWC, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, observedVWC.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, updatedVWC.Webhooks, 1, "only the generation-protection entry should survive")
	assert.Equal(t, config.GenerationLabelProtectionWebhookName, updatedVWC.Webhooks[0].Name)

	require.NoError(t, c.reconcileResourceMutatingWebhookConfiguration(ctx))
	updatedMWC, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, observedMWC.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, updatedMWC.Webhooks, "mutating webhooks have no reserved entry to keep, so all legacy entries are removed")
}

// TestReconcileResourceWebhookConfigurations_NoChurnWithoutLegacyEntries is the no-churn
// control for the prune above: a configuration that has nothing to prune must not trigger
// an Update call at all.
func TestReconcileResourceWebhookConfigurations_NoChurnWithoutLegacyEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	observedVWC := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: config.ValidatingWebhookConfigurationName},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{Name: config.GenerationLabelProtectionWebhookName},
		},
	}
	observedMWC := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: config.MutatingWebhookConfigurationName},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			mutatingWebhookWithPath("user-managed", "/some/unrelated/path"),
		},
	}

	client := kubefake.NewClientset(observedVWC.DeepCopy(), observedMWC.DeepCopy())
	vwcs := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, vwcs.Add(observedVWC))
	mwcs := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, mwcs.Add(observedMWC))
	secrets := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, secrets.Add(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: config.KyvernoNamespace()}, Data: map[string][]byte{corev1.TLSCertKey: []byte("ca")}}))

	c := &controller{
		defaultTimeout: 10, servicePort: 443, caSecretName: "ca", autoUpdateWebhooks: false,
		configuration:     config.NewDefaultConfiguration(false),
		vwcClient:         client.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		vwcLister:         admissionregistrationv1listers.NewValidatingWebhookConfigurationLister(vwcs),
		mwcClient:         client.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		mwcLister:         admissionregistrationv1listers.NewMutatingWebhookConfigurationLister(mwcs),
		secretLister:      corev1listers.NewSecretLister(secrets),
		clusterroleLister: rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}

	// First reconcile populates the generation-protection entry's full content
	// (the bare stub above isn't byte-for-byte what the builder produces). Once the
	// lister reflects that, a second reconcile should write nothing further: that's
	// the actual no-churn assertion for the legacy-prune path, since there's no
	// legacy entry here to begin with.
	require.NoError(t, c.reconcileResourceValidatingWebhookConfiguration(ctx))
	require.NoError(t, c.reconcileResourceMutatingWebhookConfiguration(ctx))
	refreshedVWC, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, observedVWC.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NoError(t, vwcs.Update(refreshedVWC))
	refreshedMWC, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, observedMWC.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NoError(t, mwcs.Update(refreshedMWC))

	client.ClearActions()
	require.NoError(t, c.reconcileResourceValidatingWebhookConfiguration(ctx))
	require.NoError(t, c.reconcileResourceMutatingWebhookConfiguration(ctx))
	for _, action := range client.Actions() {
		assert.NotEqual(t, "update", action.GetVerb(), "a configuration with nothing to prune must not be written back")
	}
}
