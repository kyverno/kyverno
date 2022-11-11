package webhook

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/toggle"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	coordinationv1informers "k8s.io/client-go/informers/coordination/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers                   = 2
	ControllerName            = "webhook-controller"
	DefaultWebhookTimeout     = 10
	AnnotationLastRequestTime = "kyverno.io/last-request-time"
	IdleDeadline              = tickerInterval * 10
	maxRetries                = 10
	managedByLabel            = "webhook.kyverno.io/managed-by"
	tickerInterval            = 10 * time.Second
)

var (
	none         = admissionregistrationv1.SideEffectClassNone
	noneOnDryRun = admissionregistrationv1.SideEffectClassNoneOnDryRun
	ifNeeded     = admissionregistrationv1.IfNeededReinvocationPolicy
	ignore       = admissionregistrationv1.Ignore
	fail         = admissionregistrationv1.Fail
	policyRule   = admissionregistrationv1.Rule{
		Resources:   []string{"clusterpolicies/*", "policies/*"},
		APIGroups:   []string{"kyverno.io"},
		APIVersions: []string{"v1", "v2beta1"},
	}
	verifyRule = admissionregistrationv1.Rule{
		Resources:   []string{"leases"},
		APIGroups:   []string{"coordination.k8s.io"},
		APIVersions: []string{"v1"},
	}
)

type controller struct {
	// clients
	discoveryClient dclient.IDiscovery
	secretClient    controllerutils.GetClient[*corev1.Secret]
	mwcClient       controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration]
	vwcClient       controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration]
	leaseClient     controllerutils.ObjectClient[*coordinationv1.Lease]
	kyvernoClient   versioned.Interface

	// listers
	mwcLister       admissionregistrationv1listers.MutatingWebhookConfigurationLister
	vwcLister       admissionregistrationv1listers.ValidatingWebhookConfigurationLister
	cpolLister      kyvernov1listers.ClusterPolicyLister
	polLister       kyvernov1listers.PolicyLister
	secretLister    corev1listers.SecretLister
	configMapLister corev1listers.ConfigMapLister
	leaseLister     coordinationv1listers.LeaseLister

	// queue
	queue workqueue.RateLimitingInterface

	// config
	server             string
	defaultTimeout     int32
	autoUpdateWebhooks bool
	admissionReports   bool
	runtime            runtimeutils.Runtime

	// state
	lock        sync.Mutex
	policyState map[string]sets.String
}

func NewController(
	discoveryClient dclient.IDiscovery,
	secretClient controllerutils.GetClient[*corev1.Secret],
	mwcClient controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration],
	vwcClient controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	leaseClient controllerutils.ObjectClient[*coordinationv1.Lease],
	kyvernoClient versioned.Interface,
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	leaseInformer coordinationv1informers.LeaseInformer,
	server string,
	defaultTimeout int32,
	autoUpdateWebhooks bool,
	admissionReports bool,
	runtime runtimeutils.Runtime,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		discoveryClient:    discoveryClient,
		secretClient:       secretClient,
		mwcClient:          mwcClient,
		vwcClient:          vwcClient,
		leaseClient:        leaseClient,
		kyvernoClient:      kyvernoClient,
		mwcLister:          mwcInformer.Lister(),
		vwcLister:          vwcInformer.Lister(),
		cpolLister:         cpolInformer.Lister(),
		polLister:          polInformer.Lister(),
		secretLister:       secretInformer.Lister(),
		configMapLister:    configMapInformer.Lister(),
		leaseLister:        leaseInformer.Lister(),
		queue:              queue,
		server:             server,
		defaultTimeout:     defaultTimeout,
		autoUpdateWebhooks: autoUpdateWebhooks,
		admissionReports:   admissionReports,
		runtime:            runtime,
		policyState: map[string]sets.String{
			config.MutatingWebhookConfigurationName:   sets.NewString(),
			config.ValidatingWebhookConfigurationName: sets.NewString(),
		},
	}
	controllerutils.AddDefaultEventHandlers(logger, mwcInformer.Informer(), queue)
	controllerutils.AddDefaultEventHandlers(logger, vwcInformer.Informer(), queue)
	controllerutils.AddEventHandlersT(
		secretInformer.Informer(),
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == tls.GenerateRootCASecretName() {
				c.enqueueAll()
			}
		},
		func(_, obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == tls.GenerateRootCASecretName() {
				c.enqueueAll()
			}
		},
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == tls.GenerateRootCASecretName() {
				c.enqueueAll()
			}
		},
	)
	controllerutils.AddEventHandlersT(
		configMapInformer.Informer(),
		func(obj *corev1.ConfigMap) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == config.KyvernoConfigMapName() {
				c.enqueueAll()
			}
		},
		func(_, obj *corev1.ConfigMap) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == config.KyvernoConfigMapName() {
				c.enqueueAll()
			}
		},
		func(obj *corev1.ConfigMap) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == config.KyvernoConfigMapName() {
				c.enqueueAll()
			}
		},
	)
	controllerutils.AddEventHandlers(
		cpolInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}) { c.enqueueResourceWebhooks(0) },
	)
	controllerutils.AddEventHandlers(
		polInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks(0) },
		func(interface{}) { c.enqueueResourceWebhooks(0) },
	)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	// add our known webhooks to the queue
	c.enqueueAll()
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile, c.watchdog)
}

func (c *controller) watchdog(ctx context.Context, logger logr.Logger) {
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
					_, err = c.leaseClient.Create(ctx, &coordinationv1.Lease{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kyverno-health",
							Namespace: config.KyvernoNamespace(),
							Labels: map[string]string{
								"app.kubernetes.io/name": kyvernov1.ValueKyvernoApp,
							},
							Annotations: map[string]string{
								AnnotationLastRequestTime: time.Now().Format(time.RFC3339),
							},
						},
					}, metav1.CreateOptions{})
					if err != nil {
						logger.Error(err, "failed to create lease")
					}
				} else {
					logger.Error(err, "failed to get lease")
				}
			} else {
				lease := lease.DeepCopy()
				lease.Labels = map[string]string{
					"app.kubernetes.io/name": kyvernov1.ValueKyvernoApp,
				}
				_, err = c.leaseClient.Update(ctx, lease, metav1.UpdateOptions{})
				if err != nil {
					logger.Error(err, "failed to update lease")
				}
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

func (c *controller) loadConfig() config.Configuration {
	cfg := config.NewDefaultConfiguration()
	cm, err := c.configMapLister.ConfigMaps(config.KyvernoNamespace()).Get(config.KyvernoConfigMapName())
	if err == nil {
		cfg.Load(cm)
	}
	return cfg
}

func (c *controller) recordPolicyState(webhookConfigurationName string, policies ...kyvernov1.PolicyInterface) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, ok := c.policyState[webhookConfigurationName]; !ok {
		return
	}
	c.policyState[webhookConfigurationName] = sets.NewString()
	for _, policy := range policies {
		policyKey, err := cache.MetaNamespaceKeyFunc(policy)
		if err != nil {
			logger.Error(err, "failed to compute policy key", "policy", policy)
		} else {
			c.policyState[webhookConfigurationName].Insert(policyKey)
		}
	}
}

func (c *controller) clientConfig(caBundle []byte, path string) admissionregistrationv1.WebhookClientConfig {
	clientConfig := admissionregistrationv1.WebhookClientConfig{
		CABundle: caBundle,
	}
	if c.server == "" {
		clientConfig.Service = &admissionregistrationv1.ServiceReference{
			Namespace: config.KyvernoNamespace(),
			Name:      config.KyvernoServiceName(),
			Path:      &path,
		}
	} else {
		url := fmt.Sprintf("https://%s%s", c.server, path)
		clientConfig.URL = &url
	}
	return clientConfig
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

func (c *controller) reconcileValidatingWebhookConfiguration(ctx context.Context, autoUpdateWebhooks bool, build func([]byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error)) error {
	caData, err := tls.ReadRootCASecret(c.secretClient)
	if err != nil {
		return err
	}
	desired, err := build(caData)
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
		w.OwnerReferences = desired.OwnerReferences
		w.Webhooks = desired.Webhooks
		return nil
	})
	return err
}

func (c *controller) reconcileMutatingWebhookConfiguration(ctx context.Context, autoUpdateWebhooks bool, build func([]byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error)) error {
	caData, err := tls.ReadRootCASecret(c.secretClient)
	if err != nil {
		return err
	}
	desired, err := build(caData)
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
		w.OwnerReferences = desired.OwnerReferences
		w.Webhooks = desired.Webhooks
		return nil
	})
	return err
}

func (c *controller) updatePolicyStatuses(ctx context.Context) error {
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
		ready := true
		for _, set := range c.policyState {
			if !set.Has(policyKey) {
				ready = false
				break
			}
		}
		status := policy.GetStatus()
		status.SetReady(ready)
		status.Autogen.Rules = nil
		if toggle.AutogenInternals.Enabled() {
			for _, rule := range autogen.ComputeRules(policy) {
				if strings.HasPrefix(rule.Name, "autogen-") {
					status.Autogen.Rules = append(status.Autogen.Rules, rule)
				}
			}
		}
		return nil
	}
	for _, policy := range policies {
		if policy.GetNamespace() == "" {
			_, err := controllerutils.UpdateStatus(
				ctx,
				policy.(*kyvernov1.ClusterPolicy),
				c.kyvernoClient.KyvernoV1().ClusterPolicies(),
				func(policy *kyvernov1.ClusterPolicy) error {
					return updateStatusFunc(policy)
				},
			)
			if err != nil {
				return err
			}
		} else {
			_, err := controllerutils.UpdateStatus(
				ctx,
				policy.(*kyvernov1.Policy),
				c.kyvernoClient.KyvernoV1().Policies(policy.GetNamespace()),
				func(policy *kyvernov1.Policy) error {
					return updateStatusFunc(policy)
				},
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	switch name {
	case config.MutatingWebhookConfigurationName:
		if c.runtime.IsRollingUpdate() {
			c.enqueueResourceWebhooks(1 * time.Second)
		} else {
			if err := c.reconcileResourceMutatingWebhookConfiguration(ctx); err != nil {
				return err
			}
			if err := c.updatePolicyStatuses(ctx); err != nil {
				return err
			}
		}
	case config.ValidatingWebhookConfigurationName:
		if c.runtime.IsRollingUpdate() {
			c.enqueueResourceWebhooks(1 * time.Second)
		} else {
			if err := c.reconcileResourceValidatingWebhookConfiguration(ctx); err != nil {
				return err
			}
			if err := c.updatePolicyStatuses(ctx); err != nil {
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

func (c *controller) buildVerifyMutatingWebhookConfiguration(caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.VerifyMutatingWebhookConfigurationName),
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:         config.VerifyMutatingWebhookName,
				ClientConfig: c.clientConfig(caBundle, config.VerifyMutatingWebhookServicePath),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: verifyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Update,
					},
				}},
				FailurePolicy:           &ignore,
				SideEffects:             &noneOnDryRun,
				ReinvocationPolicy:      &ifNeeded,
				AdmissionReviewVersions: []string{"v1beta1"},
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": kyvernov1.ValueKyvernoApp,
					},
				},
			}},
		},
		nil
}

func (c *controller) buildPolicyMutatingWebhookConfiguration(caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.PolicyMutatingWebhookConfigurationName),
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:         config.PolicyMutatingWebhookName,
				ClientConfig: c.clientConfig(caBundle, config.PolicyMutatingWebhookServicePath),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: policyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}},
				FailurePolicy:           &ignore,
				SideEffects:             &noneOnDryRun,
				ReinvocationPolicy:      &ifNeeded,
				AdmissionReviewVersions: []string{"v1beta1"},
			}},
		},
		nil
}

func (c *controller) buildPolicyValidatingWebhookConfiguration(caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.PolicyValidatingWebhookConfigurationName),
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:         config.PolicyValidatingWebhookName,
				ClientConfig: c.clientConfig(caBundle, config.PolicyValidatingWebhookServicePath),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: policyRule,
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					},
				}},
				FailurePolicy:           &ignore,
				SideEffects:             &none,
				AdmissionReviewVersions: []string{"v1beta1"},
			}},
		},
		nil
}

func (c *controller) buildDefaultResourceMutatingWebhookConfiguration(caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.MutatingWebhookConfigurationName),
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:         config.MutatingWebhookName + "-ignore",
				ClientConfig: c.clientConfig(caBundle, config.MutatingWebhookServicePath+"/ignore"),
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
				AdmissionReviewVersions: []string{"v1beta1"},
				TimeoutSeconds:          &c.defaultTimeout,
				ReinvocationPolicy:      &ifNeeded,
			}},
		},
		nil
}

func (c *controller) buildResourceMutatingWebhookConfiguration(caBundle []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	result := admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: objectMeta(config.MutatingWebhookConfigurationName),
		Webhooks:   []admissionregistrationv1.MutatingWebhook{},
	}
	if c.watchdogCheck() {
		ignore := newWebhook(c.defaultTimeout, ignore)
		fail := newWebhook(c.defaultTimeout, fail)
		policies, err := c.getAllPolicies()
		if err != nil {
			return nil, err
		}
		c.recordPolicyState(config.MutatingWebhookConfigurationName, policies...)
		// TODO: shouldn't be per failure policy, depending of the policy/rules that apply ?
		if hasWildcard(policies...) {
			ignore.setWildcard()
			fail.setWildcard()
		} else {
			for _, p := range policies {
				spec := p.GetSpec()
				if spec.HasMutate() || spec.HasVerifyImages() {
					if spec.GetFailurePolicy() == kyvernov1.Ignore {
						c.mergeWebhook(ignore, p, false)
					} else {
						c.mergeWebhook(fail, p, false)
					}
				}
			}
		}
		cfg := c.loadConfig()
		webhookCfg := config.WebhookConfig{}
		webhookCfgs := cfg.GetWebhooks()
		if len(webhookCfgs) > 0 {
			webhookCfg = webhookCfgs[0]
		}
		if !ignore.isEmpty() {
			result.Webhooks = append(
				result.Webhooks,
				admissionregistrationv1.MutatingWebhook{
					Name:         config.MutatingWebhookName + "-ignore",
					ClientConfig: c.clientConfig(caBundle, config.MutatingWebhookServicePath+"/ignore"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						ignore.buildRuleWithOperations(admissionregistrationv1.Create, admissionregistrationv1.Update),
					},
					FailurePolicy:           &ignore.failurePolicy,
					SideEffects:             &noneOnDryRun,
					AdmissionReviewVersions: []string{"v1beta1"},
					NamespaceSelector:       webhookCfg.NamespaceSelector,
					ObjectSelector:          webhookCfg.ObjectSelector,
					TimeoutSeconds:          &ignore.maxWebhookTimeout,
					ReinvocationPolicy:      &ifNeeded,
				},
			)
		}
		if !fail.isEmpty() {
			result.Webhooks = append(
				result.Webhooks,
				admissionregistrationv1.MutatingWebhook{
					Name:         config.MutatingWebhookName + "-fail",
					ClientConfig: c.clientConfig(caBundle, config.MutatingWebhookServicePath+"/fail"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						fail.buildRuleWithOperations(admissionregistrationv1.Create, admissionregistrationv1.Update),
					},
					FailurePolicy:           &fail.failurePolicy,
					SideEffects:             &noneOnDryRun,
					AdmissionReviewVersions: []string{"v1beta1"},
					NamespaceSelector:       webhookCfg.NamespaceSelector,
					ObjectSelector:          webhookCfg.ObjectSelector,
					TimeoutSeconds:          &fail.maxWebhookTimeout,
					ReinvocationPolicy:      &ifNeeded,
				},
			)
		}
	} else {
		c.recordPolicyState(config.MutatingWebhookConfigurationName)
	}
	return &result, nil
}

func (c *controller) buildDefaultResourceValidatingWebhookConfiguration(caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	sideEffects := &none
	if c.admissionReports {
		sideEffects = &noneOnDryRun
	}
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: objectMeta(config.ValidatingWebhookConfigurationName),
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:         config.ValidatingWebhookName + "-ignore",
				ClientConfig: c.clientConfig(caBundle, config.ValidatingWebhookServicePath+"/ignore"),
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
				AdmissionReviewVersions: []string{"v1beta1"},
				TimeoutSeconds:          &c.defaultTimeout,
			}},
		},
		nil
}

func (c *controller) buildResourceValidatingWebhookConfiguration(caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	result := admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: objectMeta(config.ValidatingWebhookConfigurationName),
		Webhooks:   []admissionregistrationv1.ValidatingWebhook{},
	}
	if c.watchdogCheck() {
		ignore := newWebhook(c.defaultTimeout, ignore)
		fail := newWebhook(c.defaultTimeout, fail)
		policies, err := c.getAllPolicies()
		if err != nil {
			return nil, err
		}
		c.recordPolicyState(config.ValidatingWebhookConfigurationName, policies...)
		// TODO: shouldn't be per failure policy, depending of the policy/rules that apply ?
		if hasWildcard(policies...) {
			ignore.setWildcard()
			fail.setWildcard()
		} else {
			for _, p := range policies {
				spec := p.GetSpec()
				if spec.HasValidate() || spec.HasGenerate() || spec.HasMutate() || spec.HasImagesValidationChecks() || spec.HasYAMLSignatureVerify() {
					if spec.GetFailurePolicy() == kyvernov1.Ignore {
						c.mergeWebhook(ignore, p, true)
					} else {
						c.mergeWebhook(fail, p, true)
					}
				}
			}
		}
		cfg := c.loadConfig()
		webhookCfg := config.WebhookConfig{}
		webhookCfgs := cfg.GetWebhooks()
		if len(webhookCfgs) > 0 {
			webhookCfg = webhookCfgs[0]
		}
		sideEffects := &none
		if c.admissionReports {
			sideEffects = &noneOnDryRun
		}
		if !ignore.isEmpty() {
			result.Webhooks = append(
				result.Webhooks,
				admissionregistrationv1.ValidatingWebhook{
					Name:         config.ValidatingWebhookName + "-ignore",
					ClientConfig: c.clientConfig(caBundle, config.ValidatingWebhookServicePath+"/ignore"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						ignore.buildRuleWithOperations(admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete, admissionregistrationv1.Connect),
					},
					FailurePolicy:           &ignore.failurePolicy,
					SideEffects:             sideEffects,
					AdmissionReviewVersions: []string{"v1beta1"},
					NamespaceSelector:       webhookCfg.NamespaceSelector,
					ObjectSelector:          webhookCfg.ObjectSelector,
					TimeoutSeconds:          &ignore.maxWebhookTimeout,
				},
			)
		}
		if !fail.isEmpty() {
			result.Webhooks = append(
				result.Webhooks,
				admissionregistrationv1.ValidatingWebhook{
					Name:         config.ValidatingWebhookName + "-fail",
					ClientConfig: c.clientConfig(caBundle, config.ValidatingWebhookServicePath+"/fail"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						fail.buildRuleWithOperations(admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete, admissionregistrationv1.Connect),
					},
					FailurePolicy:           &fail.failurePolicy,
					SideEffects:             sideEffects,
					AdmissionReviewVersions: []string{"v1beta1"},
					NamespaceSelector:       webhookCfg.NamespaceSelector,
					ObjectSelector:          webhookCfg.ObjectSelector,
					TimeoutSeconds:          &fail.maxWebhookTimeout,
				},
			)
		}
	} else {
		c.recordPolicyState(config.MutatingWebhookConfigurationName)
	}
	return &result, nil
}

func (c *controller) getAllPolicies() ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			policies = append(policies, cpol)
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

func (c *controller) getLease() (*coordinationv1.Lease, error) {
	return c.leaseLister.Leases(config.KyvernoNamespace()).Get("kyverno-health")
}

// mergeWebhook merges the matching kinds of the policy to webhook.rule
func (c *controller) mergeWebhook(dst *webhook, policy kyvernov1.PolicyInterface, updateValidate bool) {
	matchedGVK := make([]string, 0)
	for _, rule := range autogen.ComputeRules(policy) {
		// matching kinds in generate policies need to be added to both webhook
		if rule.HasGenerate() {
			matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
			matchedGVK = append(matchedGVK, rule.Generation.ResourceSpec.Kind)
			matchedGVK = append(matchedGVK, rule.Generation.CloneList.Kinds...)
			continue
		}
		if (updateValidate && rule.HasValidate() || rule.HasImagesValidationChecks()) ||
			(updateValidate && rule.HasMutate() && rule.IsMutateExisting()) ||
			(!updateValidate && rule.HasMutate()) && !rule.IsMutateExisting() ||
			(!updateValidate && rule.HasVerifyImages()) || (!updateValidate && rule.HasYAMLSignatureVerify()) {
			matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
		}
	}
	gvkMap := make(map[string]int)
	gvrList := make([]schema.GroupVersionResource, 0)
	for _, gvk := range matchedGVK {
		if _, ok := gvkMap[gvk]; !ok {
			gvkMap[gvk] = 1
			// NOTE: webhook stores GVR in its rules while policy stores GVK in its rules definition
			gv, k := kubeutils.GetKindFromGVK(gvk)
			switch k {
			case "Binding":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/binding"})
			case "NodeProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes/proxy"})
			case "PodAttachOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/attach"})
			case "PodExecOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/exec"})
			case "PodPortForwardOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/portforward"})
			case "PodProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/proxy"})
			case "ServiceProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services/proxy"})
			default:
				_, gvr, err := c.discoveryClient.FindResource(gv, k)
				if err != nil {
					logger.Error(err, "unable to convert GVK to GVR", "GVK", gvk)
					continue
				}
				if strings.Contains(gvk, "*") {
					group := kubeutils.GetGroupFromGVK(gvk)
					gvrList = append(gvrList, schema.GroupVersionResource{Group: group, Version: "*", Resource: gvr.Resource})
				} else {
					logger.V(4).Info("configuring webhook", "GVK", gvk, "GVR", gvr)
					gvrList = append(gvrList, gvr)
				}
			}
		}
	}
	for _, gvr := range gvrList {
		dst.groups.Insert(gvr.Group)
		if gvr.Version == "*" {
			dst.versions = sets.NewString()
			dst.versions.Insert(gvr.Version)
		} else if !dst.versions.Has("*") {
			dst.versions.Insert(gvr.Version)
		}
		dst.resources.Insert(gvr.Resource)
	}
	if dst.resources.Has("pods") {
		dst.resources.Insert("pods/ephemeralcontainers")
	}
	if dst.resources.Has("services") {
		dst.resources.Insert("services/status")
	}
	spec := policy.GetSpec()
	if spec.WebhookTimeoutSeconds != nil {
		if dst.maxWebhookTimeout < *spec.WebhookTimeoutSeconds {
			dst.maxWebhookTimeout = *spec.WebhookTimeoutSeconds
		}
	}
}
