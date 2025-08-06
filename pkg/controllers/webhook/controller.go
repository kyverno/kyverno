package webhook

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	policiesv1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1alpha1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/tls"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
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
		APIVersions: []string{"v1alpha1"},
	}
	imagevalidatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"imagevalidatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1"},
	}
	generatingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"generatingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1"},
	}
	deletingPolicyRule = admissionregistrationv1.Rule{
		Resources:   []string{"deletingpolicies"},
		APIGroups:   []string{"policies.kyverno.io"},
		APIVersions: []string{"v1alpha1"},
	}
	policyRule = admissionregistrationv1.Rule{
		Resources:   []string{"clusterpolicies", "policies"},
		APIGroups:   []string{"kyverno.io"},
		APIVersions: []string{"v1", "v2beta1"},
	}
	verifyRule = admissionregistrationv1.Rule{
		Resources:   []string{"leases"},
		APIGroups:   []string{"coordination.k8s.io"},
		APIVersions: []string{"v1"},
	}
	createUpdateDelete = []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update, kyvernov1.Delete}
	allOperations      = []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update, kyvernov1.Delete, kyvernov1.Connect}
	defaultOperations  = map[bool][]kyvernov1.AdmissionOperation{
		true:  allOperations,
		false: {kyvernov1.Create, kyvernov1.Update},
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
	cpolLister        kyvernov1listers.ClusterPolicyLister
	polLister         kyvernov1listers.PolicyLister
	vpolLister        policiesv1alpha1listers.ValidatingPolicyLister
	gpolLister        policiesv1alpha1listers.GeneratingPolicyLister
	ivpolLister       policiesv1alpha1listers.ImageValidatingPolicyLister
	mpolLister        policiesv1alpha1listers.MutatingPolicyLister
	deploymentLister  appsv1listers.DeploymentLister
	secretLister      corev1listers.SecretLister
	leaseLister       coordinationv1listers.LeaseLister
	clusterroleLister rbacv1listers.ClusterRoleLister

	// queue
	queue workqueue.TypedRateLimitingInterface[any]

	// config
	server              string
	defaultTimeout      int32
	servicePort         int32
	autoUpdateWebhooks  bool
	autoDeleteWebhooks  bool
	admissionReports    bool
	runtime             runtimeutils.Runtime
	configuration       config.Configuration
	caSecretName        string
	webhooksDeleted     bool
	webhookCleanupSetup func(context.Context, logr.Logger) error
	postWebhookCleanup  func(context.Context, logr.Logger) error

	// state
	lock        sync.Mutex
	policyState map[string]sets.Set[string]

	// stateRecorder records policies that are configured successfully in webhook object
	stateRecorder StateRecorder
}

func NewController(
	discoveryClient dclient.IDiscovery,
	mwcClient controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration],
	vwcClient controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	leaseClient controllerutils.ObjectClient[*coordinationv1.Lease],
	kyvernoClient versioned.Interface,
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	vpolInformer policiesv1alpha1informers.ValidatingPolicyInformer,
	gpolInformer policiesv1alpha1informers.GeneratingPolicyInformer,
	ivpolInformer policiesv1alpha1informers.ImageValidatingPolicyInformer,
	mpolInformer policiesv1alpha1informers.MutatingPolicyInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	secretInformer corev1informers.SecretInformer,
	leaseInformer coordinationv1informers.LeaseInformer,
	clusterroleInformer rbacv1informers.ClusterRoleInformer,
	server string,
	defaultTimeout int32,
	servicePort int32,
	webhookServerPort int32,
	autoUpdateWebhooks bool,
	autoDeleteWebhooks bool,
	admissionReports bool,
	runtime runtimeutils.Runtime,
	configuration config.Configuration,
	caSecretName string,
	webhookCleanupSetup func(context.Context, logr.Logger) error,
	postWebhookCleanup func(context.Context, logr.Logger) error,
	stateRecorder StateRecorder,
) controllers.Controller {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
	)
	c := controller{
		discoveryClient:     discoveryClient,
		mwcClient:           mwcClient,
		vwcClient:           vwcClient,
		leaseClient:         leaseClient,
		kyvernoClient:       kyvernoClient,
		mwcLister:           mwcInformer.Lister(),
		vwcLister:           vwcInformer.Lister(),
		cpolLister:          cpolInformer.Lister(),
		polLister:           polInformer.Lister(),
		vpolLister:          vpolInformer.Lister(),
		gpolLister:          gpolInformer.Lister(),
		ivpolLister:         ivpolInformer.Lister(),
		mpolLister:          mpolInformer.Lister(),
		deploymentLister:    deploymentInformer.Lister(),
		secretLister:        secretInformer.Lister(),
		leaseLister:         leaseInformer.Lister(),
		clusterroleLister:   clusterroleInformer.Lister(),
		queue:               queue,
		server:              server,
		defaultTimeout:      defaultTimeout,
		servicePort:         servicePort,
		autoUpdateWebhooks:  autoUpdateWebhooks,
		autoDeleteWebhooks:  autoDeleteWebhooks,
		admissionReports:    admissionReports,
		runtime:             runtime,
		configuration:       configuration,
		caSecretName:        caSecretName,
		webhookCleanupSetup: webhookCleanupSetup,
		postWebhookCleanup:  postWebhookCleanup,
		policyState: map[string]sets.Set[string]{
			config.MutatingWebhookConfigurationName:   sets.New[string](),
			config.ValidatingWebhookConfigurationName: sets.New[string](),
		},
		stateRecorder: stateRecorder,
	}
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
	if autoDeleteWebhooks {
		if _, err := controllerutils.AddEventHandlersT(
			deploymentInformer.Informer(),
			func(obj *appsv1.Deployment) {
			},
			func(_, obj *appsv1.Deployment) {
				if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == config.KyvernoDeploymentName() {
					c.enqueueCleanupAfter(1 * time.Second)
				}
			},
			func(obj *appsv1.Deployment) {
				if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == config.KyvernoDeploymentName() {
					c.enqueueCleanup()
				}
			},
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if _, err := controllerutils.AddEventHandlers(
		cpolInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}) { c.enqueueResourceWebhooks(0) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		polInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}) { c.enqueueResourceWebhooks(0) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		vpolInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}) { c.enqueueResourceWebhooks(0) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		gpolInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}) { c.enqueueResourceWebhooks(0) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(
		ivpolInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}) { c.enqueueResourceWebhooks(0) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	configuration.OnChanged(c.enqueueAll)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	if c.autoDeleteWebhooks {
		if err := c.webhookCleanupSetup(ctx, logger); err != nil {
			logger.Error(err, "failed to setup webhook cleanup")
		}
	}
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

func (c *controller) enqueueAll() {
	c.enqueuePolicyWebhooks()
	c.enqueueResourceWebhooks(0)
	c.enqueueVerifyWebhook()
}

func (c *controller) enqueueCleanup() {
	c.queue.Add(config.KyvernoDeploymentName())
}

func (c *controller) enqueueCleanupAfter(duration time.Duration) {
	c.queue.AddAfter(config.KyvernoDeploymentName(), duration)
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

func (c *controller) recordKyvernoPolicyState(webhookConfigurationName string, policies ...kyvernov1.PolicyInterface) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, ok := c.policyState[webhookConfigurationName]; !ok {
		return
	}
	c.policyState[webhookConfigurationName] = sets.New[string]()
	for _, policy := range policies {
		policyKey, err := cache.MetaNamespaceKeyFunc(policy)
		if err != nil {
			logger.Error(err, "failed to compute policy key", "policy", policy)
		} else {
			c.policyState[webhookConfigurationName].Insert(policyKey)
		}
	}
}

func (c *controller) recordPolicyState(policies ...engineapi.GenericPolicy) {
	for _, policy := range policies {
		if key := BuildRecorderKey(policy.GetKind(), policy.GetName()); key != "" {
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

func (c *controller) reconcileWebhookDeletion(ctx context.Context) error {
	if c.autoUpdateWebhooks {
		if c.runtime.IsGoingDown() {
			if c.webhooksDeleted {
				return nil
			}
			c.webhooksDeleted = true
			if err := c.vwcClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: kyverno.LabelWebhookManagedBy,
			}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up validating webhook configuration", "label", kyverno.LabelWebhookManagedBy)
				return err
			} else if err == nil {
				logger.V(3).Info("successfully deleted validating webhook configurations", "label", kyverno.LabelWebhookManagedBy)
			}
			if err := c.mwcClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: kyverno.LabelWebhookManagedBy,
			}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up mutating webhook configuration", "label", kyverno.LabelWebhookManagedBy)
				return err
			} else if err == nil {
				logger.V(3).Info("successfully deleted mutating webhook configurations", "label", kyverno.LabelWebhookManagedBy)
			}

			if err := c.postWebhookCleanup(ctx, logger); err != nil {
				logger.Error(err, "failed to clean up temporary rbac")
				return err
			} else {
				logger.V(3).Info("successfully deleted temporary rbac")
			}
		} else {
			if err := c.webhookCleanupSetup(ctx, logger); err != nil {
				logger.Error(err, "failed to reconcile webhook cleanup setup")
				return err
			}
			logger.V(3).Info("reconciled webhook cleanup setup")
		}
	}
	return nil
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
		return nil
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

func (c *controller) updatePolicyStatuses(ctx context.Context, webhookType string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	policies, err := c.getAllPolicies()
	if err != nil {
		return err
	}
	updateStatusFunc := func(policy kyvernov1.PolicyInterface) error {
		policyKey, err := cache.MetaNamespaceKeyFunc(policy)
		if err != nil {
			return err
		}

		spec := policy.GetSpec()
		if webhookType == config.MutatingWebhookConfigurationName {
			if !(spec.HasMutateStandard() || spec.HasVerifyImages()) {
				return nil
			}
		} else if webhookType == config.ValidatingWebhookConfigurationName {
			if !(spec.HasValidate() || spec.HasGenerate() || spec.HasMutateExisting() || spec.HasVerifyImageChecks() || spec.HasVerifyManifests()) {
				return nil
			}
		}

		ready, message := true, "Ready"
		if c.autoUpdateWebhooks {
			if set, ok := c.policyState[webhookType]; ok {
				if !set.Has(policyKey) {
					ready, message = false, "Not Ready"
				}
			}
		}
		status := policy.GetStatus()
		status.SetReady(ready, message)
		status.Autogen.Rules = nil
		rules := autogen.Default.ComputeRules(policy, "")
		setRuleCount(rules, status)
		for _, rule := range rules {
			if strings.HasPrefix(rule.Name, "autogen-") {
				status.Autogen.Rules = append(status.Autogen.Rules, rule)
			}
		}
		return nil
	}
	for _, policy := range policies {
		if policy.GetNamespace() == "" {
			err := controllerutils.UpdateStatus(
				ctx,
				policy.(*kyvernov1.ClusterPolicy),
				c.kyvernoClient.KyvernoV1().ClusterPolicies(),
				func(policy *kyvernov1.ClusterPolicy) error {
					return updateStatusFunc(policy)
				},
				func(a *kyvernov1.ClusterPolicy, b *kyvernov1.ClusterPolicy) bool {
					return datautils.DeepEqual(a.Status, b.Status)
				},
			)
			if err != nil {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					objNew, err := c.kyvernoClient.KyvernoV1().ClusterPolicies().Get(ctx, policy.GetName(), metav1.GetOptions{})
					if err != nil {
						return err
					}
					return controllerutils.UpdateStatus(
						ctx,
						objNew,
						c.kyvernoClient.KyvernoV1().ClusterPolicies(),
						func(policy *kyvernov1.ClusterPolicy) error {
							return updateStatusFunc(policy)
						},
						func(a *kyvernov1.ClusterPolicy, b *kyvernov1.ClusterPolicy) bool {
							return datautils.DeepEqual(a.Status, b.Status)
						},
					)
				})
				if retryErr != nil {
					logger.Error(retryErr, "failed to update clusterpolicy status", "policy", policy.GetName())
					continue
				}
			}
		} else {
			err := controllerutils.UpdateStatus(
				ctx,
				policy.(*kyvernov1.Policy),
				c.kyvernoClient.KyvernoV1().Policies(policy.GetNamespace()),
				func(policy *kyvernov1.Policy) error {
					return updateStatusFunc(policy)
				},
				func(a *kyvernov1.Policy, b *kyvernov1.Policy) bool {
					return datautils.DeepEqual(a.Status, b.Status)
				},
			)
			if err != nil {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					objNew, err := c.kyvernoClient.KyvernoV1().Policies(policy.GetNamespace()).Get(ctx, policy.GetName(), metav1.GetOptions{})
					if err != nil {
						return err
					}
					return controllerutils.UpdateStatus(
						ctx,
						objNew,
						c.kyvernoClient.KyvernoV1().Policies(policy.GetNamespace()),
						func(policy *kyvernov1.Policy) error {
							return updateStatusFunc(policy)
						},
						func(a *kyvernov1.Policy, b *kyvernov1.Policy) bool {
							return datautils.DeepEqual(a.Status, b.Status)
						},
					)
				})
				if retryErr != nil {
					logger.Error(retryErr, "failed to update policy status", "namespace", policy.GetNamespace(), "policy", policy.GetName())
					continue
				}
			}
		}
	}
	return nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	if c.autoDeleteWebhooks && c.runtime.IsGoingDown() {
		return c.reconcileWebhookDeletion(ctx)
	}

	switch name {
	case config.MutatingWebhookConfigurationName:
		if c.runtime.IsRollingUpdate() {
			c.enqueueResourceWebhooks(1 * time.Second)
		} else {
			if err := c.reconcileResourceMutatingWebhookConfiguration(ctx); err != nil {
				c.stateRecorder.Reset()
				return err
			}
			if err := c.updatePolicyStatuses(ctx, config.MutatingWebhookConfigurationName); err != nil {
				return err
			}
		}
	case config.ValidatingWebhookConfigurationName:
		if c.runtime.IsRollingUpdate() {
			c.enqueueResourceWebhooks(1 * time.Second)
		} else {
			if err := c.reconcileResourceValidatingWebhookConfiguration(ctx); err != nil {
				c.stateRecorder.Reset()
				return err
			}

			var errs []error
			if err := c.updatePolicyStatuses(ctx, config.ValidatingWebhookConfigurationName); err != nil {
				errs = append(errs, fmt.Errorf("failed to update policy statuses: %w", err))
			}

			return multierr.Combine(errs...)
		}
	case config.PolicyValidatingWebhookConfigurationName:
		return c.reconcilePolicyValidatingWebhookConfiguration(ctx)
	case config.PolicyMutatingWebhookConfigurationName:
		return c.reconcilePolicyMutatingWebhookConfiguration(ctx)
	case config.VerifyMutatingWebhookConfigurationName:
		return c.reconcileVerifyMutatingWebhookConfiguration(ctx)
	case config.KyvernoDeploymentName():
		return c.reconcileWebhookDeletion(ctx)
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
					Rule: imagevalidatingPolicyRule,
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
					Rule: deletingPolicyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
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

func (c *controller) buildDefaultResourceMutatingWebhookConfiguration(_ context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.MutatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:         config.MutatingWebhookName + "-ignore",
				ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.MutatingWebhookServicePath+"/ignore"),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*/*"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}},
				FailurePolicy:           &ignore,
				SideEffects:             &noneOnDryRun,
				AdmissionReviewVersions: []string{"v1"},
				TimeoutSeconds:          &c.defaultTimeout,
				ReinvocationPolicy:      &ifNeeded,
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			}, {
				Name:         config.MutatingWebhookName + "-fail",
				ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.MutatingWebhookServicePath+"/fail"),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*/*"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}},
				FailurePolicy:           &fail,
				SideEffects:             &noneOnDryRun,
				AdmissionReviewVersions: []string{"v1"},
				TimeoutSeconds:          &c.defaultTimeout,
				ReinvocationPolicy:      &ifNeeded,
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			}},
		},
		nil
}

func (c *controller) buildResourceMutatingWebhookConfiguration(ctx context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	result := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: objectMeta(config.MutatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
		Webhooks:   []admissionregistrationv1.MutatingWebhook{},
	}

	var errs []error
	if err := c.buildForPoliciesMutation(ctx, cfg, caBundle, result); err != nil {
		errs = append(errs, fmt.Errorf("failed to build webhook rules for policies: %v", err))
	}

	if err := c.buildForJSONPoliciesMutation(cfg, caBundle, result); err != nil {
		errs = append(errs, fmt.Errorf("failed to build webhook rules for imageverificationpolicies: %v", err))
	}
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
		mpols)

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
		ivpols)...)

	mutate := make([]admissionregistrationv1.MutatingWebhook, 0, len(validate))
	for _, w := range validate {
		mutate = append(mutate, admissionregistrationv1.MutatingWebhook{
			Name:                    w.Name,
			ClientConfig:            w.ClientConfig,
			FailurePolicy:           w.FailurePolicy,
			SideEffects:             w.SideEffects,
			AdmissionReviewVersions: w.AdmissionReviewVersions,
			NamespaceSelector:       w.NamespaceSelector,
			ObjectSelector:          w.ObjectSelector,
			Rules:                   w.Rules,
			MatchConditions:         w.MatchConditions,
			TimeoutSeconds:          w.TimeoutSeconds,
		})
	}
	result.Webhooks = append(result.Webhooks, mutate...)
	c.recordPolicyState(mpols...)
	return nil
}

func (c *controller) buildForPoliciesMutation(ctx context.Context, cfg config.Configuration, caBundle []byte, result *admissionregistrationv1.MutatingWebhookConfiguration) error {
	if c.watchdogCheck() {
		webhookCfg := cfg.GetWebhook()
		ignoreWebhook := newWebhook(c.defaultTimeout, ignore, cfg.GetMatchConditions())
		failWebhook := newWebhook(c.defaultTimeout, fail, cfg.GetMatchConditions())
		policies, err := c.getAllPolicies()
		if err != nil {
			return err
		}
		var fineGrainedIgnoreList, fineGrainedFailList []*webhook
		var readyPolicies []kyvernov1.PolicyInterface
		// reset policy state set
		c.recordKyvernoPolicyState(config.MutatingWebhookConfigurationName)
		for _, p := range policies {
			if p.AdmissionProcessingEnabled() {
				var ready bool
				spec := p.GetSpec()
				if spec.HasMutateStandard() || spec.HasVerifyImages() {
					if spec.CustomWebhookMatchConditions() {
						if spec.GetFailurePolicy(ctx) == kyvernov1.Ignore {
							fineGrainedIgnore := newWebhookPerPolicy(c.defaultTimeout, ignore, cfg.GetMatchConditions(), p)
							ready = c.mergeWebhook(fineGrainedIgnore, p, false)
							fineGrainedIgnoreList = append(fineGrainedIgnoreList, fineGrainedIgnore)
						} else {
							fineGrainedFail := newWebhookPerPolicy(c.defaultTimeout, fail, cfg.GetMatchConditions(), p)
							ready = c.mergeWebhook(fineGrainedFail, p, false)
							fineGrainedFailList = append(fineGrainedFailList, fineGrainedFail)
						}
					} else {
						if spec.GetFailurePolicy(ctx) == kyvernov1.Ignore {
							ready = c.mergeWebhook(ignoreWebhook, p, false)
						} else {
							ready = c.mergeWebhook(failWebhook, p, false)
						}
					}
				}
				if ready {
					readyPolicies = append(readyPolicies, p)
				}
			} else {
				readyPolicies = append(readyPolicies, p)
			}
		}
		webhooks := []*webhook{ignoreWebhook, failWebhook}
		webhooks = append(webhooks, fineGrainedIgnoreList...)
		webhooks = append(webhooks, fineGrainedFailList...)
		result.Webhooks = c.buildResourceMutatingWebhookRules(caBundle, webhookCfg, &noneOnDryRun, webhooks)
		c.recordKyvernoPolicyState(config.MutatingWebhookConfigurationName, readyPolicies...)
	} else {
		c.recordKyvernoPolicyState(config.MutatingWebhookConfigurationName)
	}
	return nil
}

func (c *controller) buildResourceMutatingWebhookRules(caBundle []byte, webhookCfg config.WebhookConfig, sideEffects *admissionregistrationv1.SideEffectClass, webhooks []*webhook) []admissionregistrationv1.MutatingWebhook {
	var mutatingWebhooks []admissionregistrationv1.MutatingWebhook //nolint:prealloc
	objectSelector := webhookCfg.ObjectSelector
	if objectSelector == nil {
		objectSelector = &metav1.LabelSelector{}
	}
	for _, webhook := range webhooks {
		if webhook.isEmpty() {
			continue
		}
		failurePolicy := webhook.failurePolicy
		timeout := capTimeout(webhook.maxWebhookTimeout)
		name, path := webhookNameAndPath(*webhook, config.MutatingWebhookName, config.MutatingWebhookServicePath)
		mutatingWebhooks = append(
			mutatingWebhooks,
			admissionregistrationv1.MutatingWebhook{
				Name:                    name,
				ClientConfig:            newClientConfig(c.server, c.servicePort, caBundle, path),
				Rules:                   webhook.buildRulesWithOperations(),
				FailurePolicy:           &failurePolicy,
				SideEffects:             sideEffects,
				AdmissionReviewVersions: []string{"v1"},
				NamespaceSelector:       webhookCfg.NamespaceSelector,
				ObjectSelector:          objectSelector,
				TimeoutSeconds:          &timeout,
				ReinvocationPolicy:      &ifNeeded,
				MatchConditions:         webhook.matchConditions,
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			},
		)
	}
	return mutatingWebhooks
}

func (c *controller) buildDefaultResourceValidatingWebhookConfiguration(_ context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	sideEffects := &none
	if c.admissionReports {
		sideEffects = &noneOnDryRun
	}
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.ValidatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:         config.ValidatingWebhookName + "-ignore",
				ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.ValidatingWebhookServicePath+"/ignore"),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*/*"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
						admissionregistrationv1.Connect,
					},
				}},
				FailurePolicy:           &ignore,
				SideEffects:             sideEffects,
				AdmissionReviewVersions: []string{"v1"},
				TimeoutSeconds:          &c.defaultTimeout,
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			}, {
				Name:         config.ValidatingWebhookName + "-fail",
				ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.ValidatingWebhookServicePath+"/fail"),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*/*"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
						admissionregistrationv1.Connect,
					},
				}},
				FailurePolicy:           &fail,
				SideEffects:             sideEffects,
				AdmissionReviewVersions: []string{"v1"},
				TimeoutSeconds:          &c.defaultTimeout,
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			}},
		},
		nil
}

func (c *controller) buildResourceValidatingWebhookConfiguration(ctx context.Context, cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: objectMeta(config.ValidatingWebhookConfigurationName, cfg.GetWebhookAnnotations(), cfg.GetWebhookLabels(), c.buildOwner()...),
		Webhooks:   []admissionregistrationv1.ValidatingWebhook{},
	}

	var errs []error
	if err := c.buildForPoliciesValidation(ctx, cfg, caBundle, webhookConfig); err != nil {
		errs = append(errs, fmt.Errorf("failed to build webhook rules for policies: %v", err))
	}

	if err := c.buildForJSONPoliciesValidation(cfg, caBundle, webhookConfig); err != nil {
		errs = append(errs, fmt.Errorf("failed to build webhook rules for validatingpolicies: %v", err))
	}

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
	result.Webhooks = append(result.Webhooks, buildWebhookRules(cfg,
		c.server,
		config.ValidatingPolicyWebhookName,
		"/vpol",
		c.servicePort,
		caBundle,
		pols)...)

	gpols, err := c.getGeneratingPolicies()
	if err != nil {
		return err
	}
	result.Webhooks = append(result.Webhooks, buildWebhookRules(cfg,
		c.server,
		config.GeneratingPolicyWebhookName,
		"/gpol",
		c.servicePort,
		caBundle,
		gpols)...)

	ivpols, err := c.getImageValidatingPolicies()
	if err != nil {
		return err
	}
	result.Webhooks = append(result.Webhooks, buildWebhookRules(cfg,
		c.server,
		config.ImageValidatingPolicyValidateWebhookName,
		"/ivpol/validate",
		c.servicePort,
		caBundle,
		ivpols)...)

	policies := append(pols, gpols...)
	policies = append(policies, ivpols...)
	c.recordPolicyState(policies...)
	return nil
}

func (c *controller) buildForPoliciesValidation(ctx context.Context, cfg config.Configuration, caBundle []byte, result *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	if c.watchdogCheck() {
		webhookCfg := cfg.GetWebhook()
		ignoreWebhook := newWebhook(c.defaultTimeout, ignore, cfg.GetMatchConditions())
		failWebhook := newWebhook(c.defaultTimeout, fail, cfg.GetMatchConditions())
		policies, err := c.getAllPolicies()
		if err != nil {
			return err
		}

		var fineGrainedIgnoreList, fineGrainedFailList []*webhook
		var readyPolicies []kyvernov1.PolicyInterface
		// reset policy state set
		c.recordKyvernoPolicyState(config.ValidatingWebhookConfigurationName)
		for _, p := range policies {
			if p.AdmissionProcessingEnabled() {
				var ready bool
				spec := p.GetSpec()
				if spec.HasValidate() || spec.HasGenerate() || spec.HasMutateExisting() || spec.HasVerifyImageChecks() || spec.HasVerifyManifests() {
					if spec.CustomWebhookMatchConditions() {
						if spec.GetFailurePolicy(ctx) == kyvernov1.Ignore {
							fineGrainedIgnore := newWebhookPerPolicy(c.defaultTimeout, ignore, cfg.GetMatchConditions(), p)
							ready = c.mergeWebhook(fineGrainedIgnore, p, true)
							fineGrainedIgnoreList = append(fineGrainedIgnoreList, fineGrainedIgnore)
						} else {
							fineGrainedFail := newWebhookPerPolicy(c.defaultTimeout, fail, cfg.GetMatchConditions(), p)
							ready = c.mergeWebhook(fineGrainedFail, p, true)
							fineGrainedFailList = append(fineGrainedFailList, fineGrainedFail)
						}
					} else {
						if spec.GetFailurePolicy(ctx) == kyvernov1.Ignore {
							ready = c.mergeWebhook(ignoreWebhook, p, true)
						} else {
							ready = c.mergeWebhook(failWebhook, p, true)
						}
					}
				}
				if ready {
					readyPolicies = append(readyPolicies, p)
				}
			} else {
				readyPolicies = append(readyPolicies, p)
			}
		}
		sideEffects := &none
		if c.admissionReports {
			sideEffects = &noneOnDryRun
		}
		webhooks := []*webhook{ignoreWebhook, failWebhook}
		webhooks = append(webhooks, fineGrainedIgnoreList...)
		webhooks = append(webhooks, fineGrainedFailList...)
		result.Webhooks = c.buildResourceValidatingWebhookRules(caBundle, webhookCfg, sideEffects, webhooks)
		c.recordKyvernoPolicyState(config.ValidatingWebhookConfigurationName, readyPolicies...)
	} else {
		c.recordKyvernoPolicyState(config.ValidatingWebhookConfigurationName)
	}
	return nil
}

func (c *controller) buildResourceValidatingWebhookRules(caBundle []byte, webhookCfg config.WebhookConfig, sideEffects *admissionregistrationv1.SideEffectClass, webhooks []*webhook) []admissionregistrationv1.ValidatingWebhook {
	var validatingWebhooks []admissionregistrationv1.ValidatingWebhook //nolint:prealloc
	objectSelector := webhookCfg.ObjectSelector
	if objectSelector == nil {
		objectSelector = &metav1.LabelSelector{}
	}
	for _, webhook := range webhooks {
		if webhook.isEmpty() {
			continue
		}
		timeout := capTimeout(webhook.maxWebhookTimeout)
		name, path := webhookNameAndPath(*webhook, config.ValidatingWebhookName, config.ValidatingWebhookServicePath)
		failurePolicy := webhook.failurePolicy
		validatingWebhooks = append(
			validatingWebhooks,
			admissionregistrationv1.ValidatingWebhook{
				Name:                    name,
				ClientConfig:            newClientConfig(c.server, c.servicePort, caBundle, path),
				Rules:                   webhook.buildRulesWithOperations(),
				FailurePolicy:           &failurePolicy,
				SideEffects:             sideEffects,
				AdmissionReviewVersions: []string{"v1"},
				NamespaceSelector:       webhookCfg.NamespaceSelector,
				ObjectSelector:          objectSelector,
				TimeoutSeconds:          &timeout,
				MatchConditions:         webhook.matchConditions,
				MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
			},
		)
	}
	return validatingWebhooks
}

func (c *controller) getAllPolicies() ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			if !cpol.GetStatus().ValidatingAdmissionPolicy.Generated {
				policies = append(policies, cpol)
			}
		}
	}
	if pols, err := c.polLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, pol)
		}
	}
	return policies, nil
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

func (c *controller) getLease() (*coordinationv1.Lease, error) {
	return c.leaseLister.Leases(config.KyvernoNamespace()).Get("kyverno-health")
}

type groupVersionResourceSubresourceScope struct {
	group       string
	version     string
	resource    string
	subresource string
	scope       admissionregistrationv1.ScopeType
}

type webhookConfig map[string]sets.Set[kyvernov1.AdmissionOperation]

func (w webhookConfig) add(kind string, ops ...kyvernov1.AdmissionOperation) {
	if len(ops) != 0 {
		if w[kind] == nil {
			w[kind] = sets.New[kyvernov1.AdmissionOperation]()
		}
		w[kind].Insert(ops...)
	}
}

func (w webhookConfig) merge(other webhookConfig) {
	for key, value := range other {
		if w[key] == nil {
			w[key] = value
		} else {
			w[key] = w[key].Union(value)
		}
	}
}

// mergeWebhook merges the matching kinds of the policy to webhook.rule
func (c *controller) mergeWebhook(dst *webhook, policy kyvernov1.PolicyInterface, updateValidate bool) (ready bool) {
	ready = true
	matched := webhookConfig{}
	for _, rule := range autogen.Default.ComputeRules(policy, "") {
		// matching kinds in generate policies need to be added to both webhooks
		if rule.HasGenerate() {
			// all four operations including CONNECT are needed for generate.
			// for example https://kyverno.io/policies/other/audit-event-on-exec/audit-event-on-exec/
			matched.merge(collectResourceDescriptions(rule, allOperations...))
			for _, g := range rule.Generation.ForEachGeneration {
				if g.GeneratePattern.ResourceSpec.Kind != "" {
					matched.add(g.GeneratePattern.ResourceSpec.Kind, createUpdateDelete...)
				} else {
					for _, kind := range g.GeneratePattern.CloneList.Kinds {
						matched.add(kind, createUpdateDelete...)
					}
				}
			}
			if rule.Generation.ResourceSpec.Kind != "" {
				matched.add(rule.Generation.ResourceSpec.Kind, createUpdateDelete...)
			} else {
				for _, kind := range rule.Generation.CloneList.Kinds {
					matched.add(kind, createUpdateDelete...)
				}
			}
		} else if (updateValidate && rule.HasValidate() || rule.HasVerifyImageChecks()) ||
			(updateValidate && rule.HasMutateExisting()) ||
			(!updateValidate && rule.HasMutateStandard()) ||
			(!updateValidate && rule.HasVerifyImages()) || (!updateValidate && rule.HasVerifyManifests()) {
			matched.merge(collectResourceDescriptions(rule, defaultOperations[updateValidate]...))
		}
	}
	for kind, ops := range matched {
		var gvrsList []groupVersionResourceSubresourceScope
		// NOTE: webhook stores GVR in its rules while policy stores GVK in its rules definition
		group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
		// if kind or group is `*` we use the scope of the policy
		policyScope := admissionregistrationv1.AllScopes
		if policy.IsNamespaced() {
			policyScope = admissionregistrationv1.NamespacedScope
		}
		// if kind is `*` no need to lookup resources
		if kind == "*" && subresource == "*" {
			gvrsList = append(gvrsList, groupVersionResourceSubresourceScope{
				group:       group,
				version:     version,
				resource:    kind,
				subresource: subresource,
				scope:       policyScope,
			})
		} else if kind == "*" && subresource == "" {
			gvrsList = append(gvrsList, groupVersionResourceSubresourceScope{
				group:       group,
				version:     version,
				resource:    kind,
				subresource: subresource,
				scope:       policyScope,
			})
		} else if kind == "*" && subresource != "" {
			gvrsList = append(gvrsList, groupVersionResourceSubresourceScope{
				group:       group,
				version:     version,
				resource:    kind,
				subresource: subresource,
				scope:       policyScope,
			})
		} else {
			gvrss, err := c.discoveryClient.FindResources(group, version, kind, subresource)
			if err != nil {
				ready = ready && false
				logger.Error(err, "unable to find resource", "group", group, "version", version, "kind", kind, "subresource", subresource)
				continue
			}
			for gvrs, resource := range gvrss {
				resourceScope := admissionregistrationv1.AllScopes
				if resource.Namespaced {
					resourceScope = admissionregistrationv1.NamespacedScope
				}
				gvrsList = append(gvrsList, groupVersionResourceSubresourceScope{
					group:       gvrs.GroupVersion.Group,
					version:     gvrs.GroupVersion.Version,
					resource:    gvrs.Resource,
					subresource: gvrs.SubResource,
					scope:       resourceScope,
				})
			}
		}
		for _, gvrs := range gvrsList {
			dst.set(gvrs.group, gvrs.version, gvrs.resource, gvrs.subresource, gvrs.scope, ops.UnsortedList()...)
		}
	}
	spec := policy.GetSpec()
	webhookTimeoutSeconds := spec.GetWebhookTimeoutSeconds()
	if webhookTimeoutSeconds != nil {
		if dst.maxWebhookTimeout < *webhookTimeoutSeconds {
			dst.maxWebhookTimeout = *webhookTimeoutSeconds
		}
	}
	return ready
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
