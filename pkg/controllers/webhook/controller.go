package webhook

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	policiesv1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/deprecations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/toggle"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	coordinationv1informers "k8s.io/client-go/informers/coordination/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

const (
	// Workers is the number of workers for this controller
	Workers                   = 2
	ControllerName            = "webhook-controller"
	DefaultWebhookTimeout     = 10
	AnnotationLastRequestTime = "kyverno.io/last-request-time"
	IdleDeadline              = tickerInterval * 10
	maxRetries                = 10
	tickerInterval            = 10 * time.Second
)

var (
	none                 = admissionregistrationv1.SideEffectClassNone
	noneOnDryRun         = admissionregistrationv1.SideEffectClassNoneOnDryRun
	ifNeeded             = admissionregistrationv1.IfNeededReinvocationPolicy
	ignore               = admissionregistrationv1.Ignore
	fail                 = admissionregistrationv1.Fail
	validatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"validatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1", "v1beta1", "v1"},
	}
	namespacedValidatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"namespacedvalidatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1beta1", "v1"},
	}
	imagevalidatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"imagevalidatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1", "v1beta1", "v1"},
	}
	namespacedimagevalidatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"namespacedimagevalidatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1beta1", "v1"},
	}
	generatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"generatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1", "v1beta1", "v1"},
	}
	namespacedGeneratingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"namespacedgeneratingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1beta1", "v1"},
	}
	mutatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"mutatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1", "v1beta1", "v1"},
	}
	namespacedMutatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"namespacedmutatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1beta1", "v1"},
	}
	deletingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"deletingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1", "v1beta1", "v1"},
	}
	// policyRule matches create and update requests for the legacy kyverno.io
	// ClusterPolicy and Policy kinds. Keep APIVersions as "v1" and "v2beta1"
	// unchanged in 1.20 so admission coverage of legacy writes is not altered;
	// the engine returns the 1.20 hard error on those writes (see
	// deprecations.BuildKindError and #17491). The legacy versions are removed
	// in 1.21. Aliased to deprecations.LegacyPolicyRule, the single source of
	// truth this value is pinned against (see legacy_apiversions_test.go),
	// deliberately kept byte-identical rather than inlined so that guard keeps
	// matching without edits.
	policyRule = deprecations.LegacyPolicyRule
	verifyRule = admissionregistrationv1.Rule{
		Resources:   []string{"leases"},
		APIGroups:   []string{"coordination.k8s.io"},
		APIVersions: []string{"v1"},
	}
)

type controller struct {
	// clients
	discoveryClient dclient.IDiscovery
	mwcClient       controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration]
	vwcClient       controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration]
	leaseClient     controllerutils.ObjectClient[*coordinationv1.Lease]
	kyvernoClient   versioned.Interface

	// listers
	mwcLister         admissionregistrationv1listers.MutatingWebhookConfigurationLister
	vwcLister         admissionregistrationv1listers.ValidatingWebhookConfigurationLister
	vpolLister        policiesv1beta1listers.ValidatingPolicyLister
	nvpolLister       policiesv1beta1listers.NamespacedValidatingPolicyLister
	gpolLister        policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister       policiesv1beta1listers.NamespacedGeneratingPolicyLister
	ivpolLister       policiesv1beta1listers.ImageValidatingPolicyLister
	nivpolLister      policiesv1beta1listers.NamespacedImageValidatingPolicyLister
	mpolLister        policiesv1beta1listers.MutatingPolicyLister
	nmpolLister       policiesv1beta1listers.NamespacedMutatingPolicyLister
	deploymentLister  appsv1listers.DeploymentLister
	secretLister      corev1listers.SecretLister
	leaseLister       coordinationv1listers.LeaseLister
	clusterroleLister rbacv1listers.ClusterRoleLister

	// queue
	queue workqueue.TypedRateLimitingInterface[any]

	// config
	server                    string
	defaultTimeout            int32
	servicePort               int32
	autoUpdateWebhooks        bool
	excludeBootstrapResources bool
	admissionReports          bool
	runtime                   runtimeutils.Runtime
	configuration             config.Configuration
	caSecretName              string

	// stateRecorder records policies that are configured successfully in webhook object
	stateRecorder StateRecorder

	celExpressionCache *expressionCache
}

func NewController(
	discoveryClient dclient.IDiscovery,
	mwcClient controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration],
	vwcClient controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	leaseClient controllerutils.ObjectClient[*coordinationv1.Lease],
	kyvernoClient versioned.Interface,
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
	vpolInformer policiesv1beta1informers.ValidatingPolicyInformer,
	nvpolInformer policiesv1beta1informers.NamespacedValidatingPolicyInformer,
	gpolInformer policiesv1beta1informers.GeneratingPolicyInformer,
	ngpolInformer policiesv1beta1informers.NamespacedGeneratingPolicyInformer,
	ivpolInformer policiesv1beta1informers.ImageValidatingPolicyInformer,
	nivpolInformer policiesv1beta1informers.NamespacedImageValidatingPolicyInformer,
	mpolInformer policiesv1beta1informers.MutatingPolicyInformer,
	nmpolInformer policiesv1beta1informers.NamespacedMutatingPolicyInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	secretInformer corev1informers.SecretInformer,
	leaseInformer coordinationv1informers.LeaseInformer,
	clusterroleInformer rbacv1informers.ClusterRoleInformer,
	server string,
	defaultTimeout int32,
	servicePort int32,
	autoUpdateWebhooks bool,
	excludeBootstrapResources bool,
	admissionReports bool,
	runtime runtimeutils.Runtime,
	configuration config.Configuration,
	caSecretName string,
	stateRecorder StateRecorder,
) controllers.Controller {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
	)
	c := controller{
		discoveryClient:           discoveryClient,
		mwcClient:                 mwcClient,
		vwcClient:                 vwcClient,
		leaseClient:               leaseClient,
		kyvernoClient:             kyvernoClient,
		mwcLister:                 mwcInformer.Lister(),
		vwcLister:                 vwcInformer.Lister(),
		vpolLister:                vpolInformer.Lister(),
		nvpolLister:               nvpolInformer.Lister(),
		gpolLister:                gpolInformer.Lister(),
		ngpolLister:               ngpolInformer.Lister(),
		ivpolLister:               ivpolInformer.Lister(),
		nivpolLister:              nivpolInformer.Lister(),
		mpolLister:                mpolInformer.Lister(),
		nmpolLister:               nmpolInformer.Lister(),
		deploymentLister:          deploymentInformer.Lister(),
		secretLister:              secretInformer.Lister(),
		leaseLister:               leaseInformer.Lister(),
		clusterroleLister:         clusterroleInformer.Lister(),
		queue:                     queue,
		server:                    server,
		defaultTimeout:            defaultTimeout,
		servicePort:               servicePort,
		autoUpdateWebhooks:        autoUpdateWebhooks,
		excludeBootstrapResources: excludeBootstrapResources,
		admissionReports:          admissionReports,
		runtime:                   runtime,
		configuration:             configuration,
		caSecretName:              caSecretName,
		stateRecorder:             stateRecorder,
		celExpressionCache:        NewExpressionCache(),
	}
	if !autoUpdateWebhooks {
		logger.Info("autoUpdateWebhooks is disabled: CEL policy webhooks require either autoUpdateWebhooks=true or an externally managed webhook configuration")
	}
	// Set up the CRD change callback
	c.discoveryClient.OnChanged(c.OnCRDChange)
	if _, _, err := controllerutils.AddDefaultEventHandlers(logger, mwcInformer.Informer(), queue); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDefaultEventHandlers(logger, vwcInformer.Informer(), queue); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		secretInformer.Informer(),
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == caSecretName {
				c.enqueueAll()
			}
		},
		func(_, obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == caSecretName {
				c.enqueueAll()
			}
		},
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == caSecretName {
				c.enqueueAll()
			}
		},
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		vpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		nvpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		mpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		nmpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		gpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		ngpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		ivpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		nivpolInformer.Informer(),
		c.handlePolicyCreate, c.handlePolicyUpdate, c.handlePolicyDelete,
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	configuration.OnChanged(c.enqueueAll)
	return &c
}

func (c *controller) handlePolicyCreate(obj interface{}) {
	if policy, ok := obj.(interface {
		GetMatchConditions() []admissionregistrationv1.MatchCondition
	}); ok {
		c.celExpressionCache.AddPolicyExpressions(policy.GetMatchConditions())
	}
	c.enqueueResourceWebhooks(0)
}

func (c *controller) handlePolicyUpdate(oldObj, newObj interface{}) {
	if oldPolicy, ok := oldObj.(interface {
		GetMatchConditions() []admissionregistrationv1.MatchCondition
	}); ok {
		c.celExpressionCache.RemovePolicyExpressions(oldPolicy.GetMatchConditions())
	}
	if newPolicy, ok := newObj.(interface {
		GetMatchConditions() []admissionregistrationv1.MatchCondition
	}); ok {
		c.celExpressionCache.AddPolicyExpressions(newPolicy.GetMatchConditions())
	}
	c.enqueueResourceWebhooks(0)
}

func (c *controller) handlePolicyDelete(obj interface{}) {
	if policy, ok := obj.(interface {
		GetMatchConditions() []admissionregistrationv1.MatchCondition
	}); ok {
		c.celExpressionCache.RemovePolicyExpressions(policy.GetMatchConditions())
	}
	c.enqueueResourceWebhooks(0)
}

func (c *controller) Run(ctx context.Context, workers int) {
	// add our known webhooks to the queue
	c.enqueueAll()
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile, c.watchdog)
}

func (c *controller) createLease(ctx context.Context) error {
	_, err := c.leaseClient.Create(ctx, &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-health",
			Namespace: config.KyvernoNamespace(),
			Labels: map[string]string{
				"app.kubernetes.io/name": kyverno.ValueKyvernoApp,
			},
			Annotations: map[string]string{
				AnnotationLastRequestTime: time.Now().Format(time.RFC3339),
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *controller) watchdog(ctx context.Context, logger logr.Logger) {
	_, err := c.getLease()
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := c.createLease(ctx); err != nil {
				logger.Error(err, "failed to create lease at initial setup")
			}
		} else {
			logger.Error(err, "failed to get lease")
		}
	}

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lease, err := c.getLease()
			if err != nil {
				if apierrors.IsNotFound(err) {
					if err := c.createLease(ctx); err != nil {
						logger.Error(err, "failed to create lease")
					}
					continue
				}
				logger.Error(err, "failed to get lease during update")
				continue
			}
			leaseCopy := lease.DeepCopy()
			leaseCopy.Labels = map[string]string{
				"app.kubernetes.io/name": kyverno.ValueKyvernoApp,
			}
			leaseCopy.Annotations = map[string]string{
				AnnotationLastRequestTime: time.Now().Format(time.RFC3339),
			}
			_, err = c.leaseClient.Update(ctx, leaseCopy, metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err, "failed to update lease")
			}
			c.enqueueResourceWebhooks(0)
		}
	}
}

func (c *controller) watchdogCheck() bool {
	lease, err := c.getLease()
	if err != nil {
		logger.Error(err, "failed to get lease")
		return false
	}
	annotations := lease.GetAnnotations()
	if annotations == nil {
		return false
	}
	annTime, err := time.Parse(time.RFC3339, annotations[AnnotationLastRequestTime])
	if err != nil {
		return false
	}
	return time.Now().Before(annTime.Add(IdleDeadline))
}

func (c *controller) OnCRDChange() {
	logger := logger.WithName("crd-change-handler")
	logger.Info("CRD change detected, re-evaluating all policies")
	c.enqueueAll()
}

func (c *controller) enqueueAll() {
	c.enqueuePolicyWebhooks()
	c.enqueueResourceWebhooks(0)
	c.enqueueVerifyWebhook()
}

func (c *controller) enqueuePolicyWebhooks() {
	c.queue.Add(config.PolicyValidatingWebhookConfigurationName)
	c.queue.Add(config.PolicyMutatingWebhookConfigurationName)
}

func (c *controller) enqueueResourceWebhooks(duration time.Duration) {
	c.queue.AddAfter(config.MutatingWebhookConfigurationName, duration)
	c.queue.AddAfter(config.ValidatingWebhookConfigurationName, duration)
}

func (c *controller) enqueueVerifyWebhook() {
	c.queue.Add(config.VerifyMutatingWebhookConfigurationName)
}

func (c *controller) recordPolicyState(policies ...engineapi.GenericPolicy) {
	for _, policy := range policies {
		if key := BuildRecorderKey(policy.GetKind(), policy.GetName(), policy.GetNamespace()); key != "" {
			c.stateRecorder.Record(key)
		}
	}
}

func (c *controller) reconcileResourceValidatingWebhookConfiguration(ctx context.Context) error {
	if c.autoUpdateWebhooks {
		return c.reconcileValidatingWebhookConfiguration(ctx, c.autoUpdateWebhooks, c.buildResourceValidatingWebhookConfiguration)
	} else {
		return c.reconcileValidatingWebhookConfiguration(ctx, c.autoUpdateWebhooks, c.buildDefaultResourceValidatingWebhookConfiguration)
	}
}

func (c *controller) reconcileResourceMutatingWebhookConfiguration(ctx context.Context) error {
	if c.autoUpdateWebhooks {
		return c.reconcileMutatingWebhookConfiguration(ctx, c.autoUpdateWebhooks, c.buildResourceMutatingWebhookConfiguration)
	} else {
		return c.reconcileMutatingWebhookConfiguration(ctx, c.autoUpdateWebhooks, c.buildDefaultResourceMutatingWebhookConfiguration)
	}
}

func (c *controller) reconcilePolicyValidatingWebhookConfiguration(ctx context.Context) error {
	return c.reconcileValidatingWebhookConfiguration(ctx, true, c.buildPolicyValidatingWebhookConfiguration)
}

func (c *controller) reconcilePolicyMutatingWebhookConfiguration(ctx context.Context) error {
	return c.reconcileMutatingWebhookConfiguration(ctx, true, c.buildPolicyMutatingWebhookConfiguration)
}

func (c *controller) reconcileVerifyMutatingWebhookConfiguration(ctx context.Context) error {
	return c.reconcileMutatingWebhookConfiguration(ctx, true, c.buildVerifyMutatingWebhookConfiguration)
}

func (c *controller) reconcileValidatingWebhookConfiguration(ctx context.Context, autoUpdateWebhooks bool, build func(context.Context, config.Configuration, []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error)) error {
	caData, err := tls.ReadRootCASecret(c.caSecretName, config.KyvernoNamespace(), c.secretLister.Secrets(config.KyvernoNamespace()))
	if err != nil {
		return err
	}
	desired, err := build(ctx, c.configuration, caData)
	if err != nil {
		return err
	}
	observed, err := c.vwcLister.Get(desired.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.vwcClient.Create(ctx, desired, metav1.CreateOptions{})
			return err
		}
		return err
	}
	if !autoUpdateWebhooks {
		if desired.Name == config.ValidatingWebhookConfigurationName {
			_, err := controllerutils.Update(ctx, observed, c.vwcClient, func(w *admissionregistrationv1.ValidatingWebhookConfiguration) error {
				w.Webhooks = pruneLegacyWebhooks(w.Webhooks, config.ValidatingWebhookServicePath)
				if toggle.FromContext(ctx).ProtectGenerationLabels() {
					setGenerationLabelProtectionWebhook(w, c.buildGenerationLabelProtectionWebhook(ctx, caData))
				} else {
					w.Webhooks = removeGenerationLabelProtectionWebhook(w.Webhooks)
				}
				return nil
			})
			return err
		}
		return nil
	}
	_, err = controllerutils.Update(ctx, observed, c.vwcClient, func(w *admissionregistrationv1.ValidatingWebhookConfiguration) error {
		w.Labels = desired.Labels
		w.Annotations = desired.Annotations
		w.OwnerReferences = desired.OwnerReferences
		w.Webhooks = desired.Webhooks
		return nil
	})
	return err
}

func (c *controller) reconcileMutatingWebhookConfiguration(ctx context.Context, autoUpdateWebhooks bool, build func(context.Context, config.Configuration, []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error)) error {
	caData, err := tls.ReadRootCASecret(c.caSecretName, config.KyvernoNamespace(), c.secretLister.Secrets(config.KyvernoNamespace()))
	if err != nil {
		return err
	}
	desired, err := build(ctx, c.configuration, caData)
	if err != nil {
		return err
	}
	observed, err := c.mwcLister.Get(desired.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.mwcClient.Create(ctx, desired, metav1.CreateOptions{})
			return err
		}
		return err
	}
	if !autoUpdateWebhooks {
		_, err := controllerutils.Update(ctx, observed, c.mwcClient, func(w *admissionregistrationv1.MutatingWebhookConfiguration) error {
			w.Webhooks = pruneLegacyMutatingWebhooks(w.Webhooks, config.MutatingWebhookServicePath)
			return nil
		})
		return err
	}
	_, err = controllerutils.Update(ctx, observed, c.mwcClient, func(w *admissionregistrationv1.MutatingWebhookConfiguration) error {
		w.Labels = desired.Labels
		w.Annotations = desired.Annotations
		w.OwnerReferences = desired.OwnerReferences
		w.Webhooks = desired.Webhooks
		return nil
	})
	return err
}

// pruneLegacyWebhooks drops any validating webhook whose service path is servicePath or a servicePath/...
// subpath. Used in the autoUpdateWebhooks=false branches, where a webhook configuration persisted by 1.19
// (or managed externally) may still carry the removed legacy resource admission entries; matching by path
// rather than name because operators own webhook names in this mode. Returns webhooks unchanged, same
// slice value (nil included) when nothing matches servicePath: the lister's observed slice is nil for
// an already-clean config (omitempty), and controllerutils.Update compares with reflect.DeepEqual, so
// returning a freshly allocated empty slice here would differ from nil and write forever on an
// autoUpdateWebhooks=false leader's 10s ticker.
func pruneLegacyWebhooks(webhooks []admissionregistrationv1.ValidatingWebhook, servicePath string) []admissionregistrationv1.ValidatingWebhook {
	var pruned []admissionregistrationv1.ValidatingWebhook
	removed := false
	for i, w := range webhooks {
		if isLegacyWebhookPath(w.ClientConfig.Service, servicePath) {
			if !removed {
				pruned = append(pruned, webhooks[:i]...)
				removed = true
			}
			continue
		}
		if removed {
			pruned = append(pruned, w)
		}
	}
	if !removed {
		return webhooks
	}
	return pruned
}

// pruneLegacyMutatingWebhooks is pruneLegacyWebhooks for MutatingWebhook; see that function's comment.
func pruneLegacyMutatingWebhooks(webhooks []admissionregistrationv1.MutatingWebhook, servicePath string) []admissionregistrationv1.MutatingWebhook {
	var pruned []admissionregistrationv1.MutatingWebhook
	removed := false
	for i, w := range webhooks {
		if isLegacyWebhookPath(w.ClientConfig.Service, servicePath) {
			if !removed {
				pruned = append(pruned, webhooks[:i]...)
				removed = true
			}
			continue
		}
		if removed {
			pruned = append(pruned, w)
		}
	}
	if !removed {
		return webhooks
	}
	return pruned
}

func isLegacyWebhookPath(service *admissionregistrationv1.ServiceReference, servicePath string) bool {
	if service == nil || service.Path == nil {
		return false
	}
	path := *service.Path
	return path == servicePath || strings.HasPrefix(path, servicePath+"/")
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	switch name {
	case config.MutatingWebhookConfigurationName:
		if c.runtime.IsRollingUpdate() {
			c.enqueueResourceWebhooks(1 * time.Second)
		} else {
			if !c.watchdogCheck() {
				// Health not confirmed (startup, leader change, resumed cluster):
				// rebuilding now would publish an empty webhook configuration and drop
				// every rule. Requeue and keep the persisted configuration until the
				// watchdog confirms health (it refreshes on its own ticker).
				c.enqueueResourceWebhooks(1 * time.Second)
			} else if err := c.reconcileResourceMutatingWebhookConfiguration(ctx); err != nil {
				c.stateRecorder.Reset()
				return err
			}
		}
	case config.ValidatingWebhookConfigurationName:
		if c.runtime.IsRollingUpdate() {
			c.enqueueResourceWebhooks(1 * time.Second)
		} else {
			if !c.watchdogCheck() {
				// Health not confirmed (startup, leader change, resumed cluster):
				// rebuilding now would publish an empty webhook configuration and drop
				// every rule. Requeue and keep the persisted configuration until the
				// watchdog confirms health (it refreshes on its own ticker).
				c.enqueueResourceWebhooks(1 * time.Second)
			} else if err := c.reconcileResourceValidatingWebhookConfiguration(ctx); err != nil {
				c.stateRecorder.Reset()
				return err
			}
		}
	case config.PolicyValidatingWebhookConfigurationName:
		return c.reconcilePolicyValidatingWebhookConfiguration(ctx)
	case config.PolicyMutatingWebhookConfigurationName:
		return c.reconcilePolicyMutatingWebhookConfiguration(ctx)
	case config.VerifyMutatingWebhookConfigurationName:
		return c.reconcileVerifyMutatingWebhookConfiguration(ctx)
	}
	return nil
}

func (c *controller) buildVerifyMutatingWebhookConfiguration(_ context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.VerifyMutatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:         config.VerifyMutatingWebhookName,
				ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.VerifyMutatingWebhookServicePath),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: verifyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Update,
					},
				}},
				FailurePolicy:           &ignore,
				SideEffects:             &noneOnDryRun,
				TimeoutSeconds:          &c.defaultTimeout,
				ReinvocationPolicy:      &ifNeeded,
				AdmissionReviewVersions: []string{"v1"},
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": kyverno.ValueKyvernoApp,
					},
				},
				MatchPolicy: ptr.To(admissionregistrationv1.Equivalent),
			}},
		},
		nil
}

func (c *controller) buildPolicyMutatingWebhookConfiguration(_ context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.PolicyMutatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:         config.PolicyMutatingWebhookName,
				ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.PolicyMutatingWebhookServicePath),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: policyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}},
				FailurePolicy:           &fail,
				TimeoutSeconds:          &c.defaultTimeout,
				SideEffects:             &noneOnDryRun,
				ReinvocationPolicy:      &ifNeeded,
				AdmissionReviewVersions: []string{"v1"},
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			}},
		},
		nil
}

func (c *controller) buildPolicyValidatingWebhookConfiguration(_ context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.PolicyValidatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:         config.PolicyValidatingWebhookName,
				ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.PolicyValidatingWebhookServicePath),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: policyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: validatingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: namespacedValidatingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: imagevalidatingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: namespacedimagevalidatingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: generatingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: namespacedGeneratingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: mutatingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: namespacedMutatingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					Rule: deletingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}, {
					// Legacy ClusterPolicy/Policy status writes (Kyverno's own controllers,
					// transitionally allowed by deprecations.AllowSubresourceForLegacyStatusWriter)
					// must reach this same gate; policyRule above only matches the base resource.
					Rule:       deprecations.LegacyPolicyStatusRule,
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
				}},
				FailurePolicy:           &fail,
				TimeoutSeconds:          &c.defaultTimeout,
				SideEffects:             &none,
				AdmissionReviewVersions: []string{"v1"},
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			}},
		},
		nil
}

// buildDefaultResourceMutatingWebhookConfiguration is the static configuration used
// when autoUpdateWebhooks is false. The legacy catch-all "/mutate" ignore/fail
// webhooks are gone along with that route, so there is nothing resource-facing left
// to register here; mutation is served entirely by the CEL (auto-updated) path and
// the dedicated verify webhook.
func (c *controller) buildDefaultResourceMutatingWebhookConfiguration(_ context.Context, cfg config.Configuration, _ []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.MutatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
			Webhooks:   []admissionregistrationv1.MutatingWebhook{},
		},
		nil
}

func (c *controller) buildResourceMutatingWebhookConfiguration(ctx context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	result := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: objectMeta(config.MutatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
		Webhooks:   []admissionregistrationv1.MutatingWebhook{},
	}

	var errs []error
	if err := c.buildForJSONPoliciesMutation(cfg, caBundle, result); err != nil {
		errs = append(errs, fmt.Errorf("failed to build webhook rules for imageverificationpolicies: %v", err))
	}

	slices.SortFunc(result.Webhooks, func(a, b admissionregistrationv1.MutatingWebhook) int {
		return strings.Compare(a.Name, b.Name)
	})

	excludeBootstrapResourcesFromMutatingWebhooks(result.Webhooks, c.excludeBootstrapResources)

	return result, multierr.Combine(errs...)
}

func (c *controller) buildForJSONPoliciesMutation(cfg config.Configuration, caBundle []byte, result *admissionregistrationv1.MutatingWebhookConfiguration) error {
	if !c.watchdogCheck() {
		return nil
	}

	mpols, err := c.getMutatingPolicies()
	if err != nil {
		return err
	}

	validate := buildWebhookRules(cfg,
		c.server,
		config.MutatingPolicyWebhookName,
		"/mpol",
		c.servicePort,
		caBundle,
		mpols,
		c.celExpressionCache)

	nmpols, err := c.getNamespacedMutatingPolicies()
	if err != nil {
		return err
	}

	validate = append(validate, buildWebhookRules(cfg,
		c.server,
		config.NamespacedMutatingPolicyWebhookName,
		"/nmpol",
		c.servicePort,
		caBundle,
		nmpols,
		c.celExpressionCache)...)
	// Only the (namespaced) MutatingPolicy webhooks are reinvoked; the image
	// verification webhooks below keep the API default.
	mutatingPolicyWebhooks := len(validate)

	ivpols, err := c.getImageValidatingPolicies()
	if err != nil {
		return err
	}

	validate = append(validate, buildWebhookRules(cfg,
		c.server,
		config.ImageValidatingPolicyMutateWebhookName,
		"/ivpol/mutate",
		c.servicePort,
		caBundle,
		ivpolsNeedingMutation(ivpols),
		c.celExpressionCache)...)

	nivpols, err := c.getNamespacedImageValidatingPolicies()
	if err != nil {
		return err
	}

	validate = append(validate, buildWebhookRules(cfg,
		c.server,
		config.NamespacedImageValidatingPolicyMutateWebhookName,
		"/nivpol/mutate",
		c.servicePort,
		caBundle,
		ivpolsNeedingMutation(nivpols),
		c.celExpressionCache)...)

	mutate := make([]admissionregistrationv1.MutatingWebhook, 0, len(validate))
	for i, w := range validate {
		var reinvocationPolicy *admissionregistrationv1.ReinvocationPolicyType
		if i < mutatingPolicyWebhooks {
			reinvocationPolicy = &ifNeeded
		}
		mutate = append(mutate, admissionregistrationv1.MutatingWebhook{
			Name:                    w.Name,
			ClientConfig:            w.ClientConfig,
			FailurePolicy:           w.FailurePolicy,
			SideEffects:             w.SideEffects,
			AdmissionReviewVersions: w.AdmissionReviewVersions,
			NamespaceSelector:       w.NamespaceSelector,
			ObjectSelector:          w.ObjectSelector,
			Rules:                   sortedRules(deDuplicatedRules(w.Rules)),
			MatchConditions:         w.MatchConditions,
			MatchPolicy:             w.MatchPolicy,
			TimeoutSeconds:          w.TimeoutSeconds,
			ReinvocationPolicy:      reinvocationPolicy,
		})
	}
	result.Webhooks = append(result.Webhooks, mutate...)
	c.recordPolicyState(append(mpols, nmpols...)...)
	return nil
}

// buildDefaultResourceValidatingWebhookConfiguration is the static configuration used
// when autoUpdateWebhooks is false. The legacy catch-all "/validate" ignore/fail
// webhooks are gone along with that route; only the generation-label protection
// webhook (unrelated to the removed catch-all) remains resource-facing here.
func (c *controller) buildDefaultResourceValidatingWebhookConfiguration(ctx context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	var webhooks []admissionregistrationv1.ValidatingWebhook
	if toggle.FromContext(ctx).ProtectGenerationLabels() {
		webhooks = append(webhooks, c.buildGenerationLabelProtectionWebhook(ctx, caBundle))
	}
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.ValidatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
			Webhooks:   webhooks,
		},
		nil
}

func (c *controller) buildResourceValidatingWebhookConfiguration(ctx context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: objectMeta(config.ValidatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
		Webhooks:   []admissionregistrationv1.ValidatingWebhook{},
	}

	var errs []error
	if err := c.buildForJSONPoliciesValidation(cfg, caBundle, webhookConfig); err != nil {
		errs = append(errs, fmt.Errorf("failed to build webhook rules for validatingpolicies: %v", err))
	}

	// Metadata protection is independent of policy selectors and applies the same
	// configured bootstrap exclusion. Its association selector skips ordinary resources.
	excludeBootstrapResourcesFromValidatingWebhooks(webhookConfig.Webhooks, c.excludeBootstrapResources)
	if toggle.FromContext(ctx).ProtectGenerationLabels() {
		webhookConfig.Webhooks = append(webhookConfig.Webhooks, c.buildGenerationLabelProtectionWebhook(ctx, caBundle))
	}

	slices.SortFunc(webhookConfig.Webhooks, func(a, b admissionregistrationv1.ValidatingWebhook) int {
		return strings.Compare(a.Name, b.Name)
	})

	return webhookConfig, multierr.Combine(errs...)
}

func (c *controller) buildForJSONPoliciesValidation(cfg config.Configuration, caBundle []byte, result *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	if !c.watchdogCheck() {
		return nil
	}

	pols, err := c.getValidatingPolicies()
	if err != nil {
		return err
	}
	vpolWebhooks := buildWebhookRules(cfg,
		c.server,
		config.ValidatingPolicyWebhookName,
		"/vpol",
		c.servicePort,
		caBundle,
		pols,
		c.celExpressionCache)

	for i := range vpolWebhooks {
		vpolWebhooks[i].Rules = sortedRules(deDuplicatedRules(vpolWebhooks[i].Rules))
	}
	result.Webhooks = append(result.Webhooks, vpolWebhooks...)

	nvpols, err := c.getNamespacedValidatingPolicies()
	if err != nil {
		return err
	}
	nvpolWebhooks := buildWebhookRules(cfg,
		c.server,
		config.NamespacedValidatingPolicyWebhookName,
		"/nvpol",
		c.servicePort,
		caBundle,
		nvpols,
		c.celExpressionCache)

	for i := range nvpolWebhooks {
		nvpolWebhooks[i].Rules = sortedRules(deDuplicatedRules(nvpolWebhooks[i].Rules))
	}
	result.Webhooks = append(result.Webhooks, nvpolWebhooks...)

	gpols, err := c.getGeneratingPolicies()
	if err != nil {
		return err
	}
	gpolWebhooks := buildWebhookRules(cfg,
		c.server,
		config.GeneratingPolicyWebhookName,
		"/gpol",
		c.servicePort,
		caBundle,
		gpols,
		c.celExpressionCache)

	for i := range gpolWebhooks {
		gpolWebhooks[i].Rules = sortedRules(deDuplicatedRules(gpolWebhooks[i].Rules))
	}
	result.Webhooks = append(result.Webhooks, gpolWebhooks...)

	ngpols, err := c.getNamespacedGeneratingPolicies()
	if err != nil {
		return err
	}
	ngpolWebhooks := buildWebhookRules(cfg,
		c.server,
		config.NamespacedGeneratingPolicyWebhookName,
		"/ngpol",
		c.servicePort,
		caBundle,
		ngpols,
		c.celExpressionCache)

	for i := range ngpolWebhooks {
		ngpolWebhooks[i].Rules = sortedRules(deDuplicatedRules(ngpolWebhooks[i].Rules))
	}
	result.Webhooks = append(result.Webhooks, ngpolWebhooks...)

	ivpols, err := c.getImageValidatingPolicies()
	if err != nil {
		return err
	}
	ivpolWebhooks := buildWebhookRules(cfg,
		c.server,
		config.ImageValidatingPolicyValidateWebhookName,
		"/ivpol/validate",
		c.servicePort,
		caBundle,
		ivpols,
		c.celExpressionCache)

	for i := range ivpolWebhooks {
		ivpolWebhooks[i].Rules = sortedRules(deDuplicatedRules(ivpolWebhooks[i].Rules))
	}
	result.Webhooks = append(result.Webhooks, ivpolWebhooks...)

	nivpols, err := c.getNamespacedImageValidatingPolicies()
	if err != nil {
		return err
	}
	result.Webhooks = append(result.Webhooks, buildWebhookRules(cfg,
		c.server,
		config.NamespacedImageValidatingPolicyValidateWebhookName,
		"/nivpol/validate",
		c.servicePort,
		caBundle,
		nivpols,
		c.celExpressionCache)...)

	policies := append(pols, nvpols...)
	policies = append(policies, gpols...)
	policies = append(policies, ngpols...)
	policies = append(policies, ivpols...)
	policies = append(policies, nivpols...)
	c.recordPolicyState(policies...)
	return nil
}

func (c *controller) getValidatingPolicies() ([]engineapi.GenericPolicy, error) {
	validatingpolicies, err := c.vpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	vpols := make([]engineapi.GenericPolicy, 0)
	for _, vpol := range validatingpolicies {
		if vpol.Spec.AdmissionEnabled() && !vpol.GetStatus().Generated {
			vpols = append(vpols, engineapi.NewValidatingPolicy(vpol))
		}
	}
	return vpols, nil
}

func (c *controller) getNamespacedValidatingPolicies() ([]engineapi.GenericPolicy, error) {
	namespacedvalidatingpolicies, err := c.nvpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	nvpols := make([]engineapi.GenericPolicy, 0)
	for _, nvpol := range namespacedvalidatingpolicies {
		if nvpol.Spec.AdmissionEnabled() && !nvpol.GetStatus().Generated {
			nvpols = append(nvpols, engineapi.NewNamespacedValidatingPolicy(nvpol))
		}
	}
	return nvpols, nil
}

func (c *controller) getGeneratingPolicies() ([]engineapi.GenericPolicy, error) {
	generatingpolicies, err := c.gpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	gpols := make([]engineapi.GenericPolicy, 0)
	for _, gpol := range generatingpolicies {
		if gpol.Spec.AdmissionEnabled() {
			gpols = append(gpols, engineapi.NewGeneratingPolicy(gpol))
		}
	}
	return gpols, nil
}

func (c *controller) getNamespacedGeneratingPolicies() ([]engineapi.GenericPolicy, error) {
	namespacedgeneratingpolicies, err := c.ngpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	ngpols := make([]engineapi.GenericPolicy, 0)
	for _, ngpol := range namespacedgeneratingpolicies {
		if ngpol.Spec.AdmissionEnabled() {
			ngpols = append(ngpols, engineapi.NewNamespacedGeneratingPolicy(ngpol))
		}
	}
	return ngpols, nil
}

func (c *controller) getImageValidatingPolicies() ([]engineapi.GenericPolicy, error) {
	policies, err := c.ivpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	ivpols := make([]engineapi.GenericPolicy, 0)
	for _, ivpol := range policies {
		if ivpol.Spec.AdmissionEnabled() {
			ivpols = append(ivpols, engineapi.NewImageValidatingPolicy(ivpol))
		}
	}
	return ivpols, nil
}

func (c *controller) getNamespacedImageValidatingPolicies() ([]engineapi.GenericPolicy, error) {
	policies, err := c.nivpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	nivpols := make([]engineapi.GenericPolicy, 0)
	for _, nivpol := range policies {
		if nivpol.Spec.AdmissionEnabled() {
			nivpols = append(nivpols, engineapi.NewNamespacedImageValidatingPolicy(nivpol))
		}
	}
	return nivpols, nil
}

// ivpolsNeedingMutation filters ivpol/nivpol policies to those that actually
// require a mutating webhook, i.e. those with MutateDigest enabled (it defaults
// to true when nil, so an unset spec always qualifies).
//
// VerifyDigest is deliberately not considered here: digest pinning is the only
// mutation the ivpol mutating webhook performs. Asserting that an image carries
// a digest is a validation concern handled by the validating webhook (mirroring
// v1, where VerifyDigest is enforced in the validate_image handler), so a policy
// with mutateDigest disabled and verifyDigest enabled would otherwise get a
// mutating webhook that can never produce a patch.
func ivpolsNeedingMutation(policies []engineapi.GenericPolicy) []engineapi.GenericPolicy {
	result := make([]engineapi.GenericPolicy, 0, len(policies))
	for _, p := range policies {
		ivpol := p.AsImageValidatingPolicyLike()
		if ivpol == nil {
			continue
		}
		spec := ivpol.GetSpec()
		if spec.ValidationConfigurations.MutateDigest == nil || *spec.ValidationConfigurations.MutateDigest {
			result = append(result, p)
		}
	}
	return result
}

func (c *controller) getMutatingPolicies() ([]engineapi.GenericPolicy, error) {
	policies, err := c.mpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	mpols := make([]engineapi.GenericPolicy, 0)
	for _, mpol := range policies {
		if mpol.Spec.AdmissionEnabled() && !mpol.GetStatus().Generated {
			mpols = append(mpols, engineapi.NewMutatingPolicy(mpol))
		}
	}
	return mpols, nil
}

func (c *controller) getNamespacedMutatingPolicies() ([]engineapi.GenericPolicy, error) {
	policies, err := c.nmpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	nmpols := make([]engineapi.GenericPolicy, 0)
	for _, nmpol := range policies {
		if nmpol.Spec.AdmissionEnabled() && !nmpol.GetStatus().Generated {
			nmpols = append(nmpols, engineapi.NewNamespacedMutatingPolicy(nmpol))
		}
	}
	return nmpols, nil
}

func (c *controller) getLease() (*coordinationv1.Lease, error) {
	return c.leaseLister.Leases(config.KyvernoNamespace()).Get("kyverno-health")
}

func (c *controller) buildOwner() []metav1.OwnerReference {
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		kyverno.LabelAppComponent: "kyverno",
	}))

	clusterroles, err := c.clusterroleLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to fetch kyverno clusterroles, won't set owners for webhook configurations")
		return nil
	}

	for _, clusterrole := range clusterroles {
		if wildcard.Match("*:webhook", clusterrole.GetName()) {
			return []metav1.OwnerReference{
				{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
					Name:       clusterrole.GetName(),
					UID:        clusterrole.GetUID(),
				},
			}
		}
	}
	return nil
}
