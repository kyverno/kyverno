package background

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/tls"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 2
	ControllerName = "webhook-ca-controller"
	maxRetries     = 10
	managedByLabel = "webhook.kyverno.io/managed-by"
)

var (
	noneOnDryRun = admissionregistrationv1.SideEffectClassNoneOnDryRun
	ifNeeded     = admissionregistrationv1.IfNeededReinvocationPolicy
	ignore       = admissionregistrationv1.Ignore
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
	secretClient controllerutils.GetClient[*corev1.Secret]
	mwcClient    controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration]
	vwcClient    controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration]

	// listers
	secretLister    corev1listers.SecretLister
	configMapLister corev1listers.ConfigMapLister
	mwcLister       admissionregistrationv1listers.MutatingWebhookConfigurationLister
	vwcLister       admissionregistrationv1listers.ValidatingWebhookConfigurationLister

	// queue
	queue workqueue.RateLimitingInterface

	// config
	server string
}

func NewController(
	secretClient controllerutils.GetClient[*corev1.Secret],
	mwcClient controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration],
	vwcClient controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		secretClient:    secretClient,
		mwcClient:       mwcClient,
		vwcClient:       vwcClient,
		secretLister:    secretInformer.Lister(),
		configMapLister: configMapInformer.Lister(),
		mwcLister:       mwcInformer.Lister(),
		vwcLister:       vwcInformer.Lister(),
		queue:           queue,
	}
	controllerutils.AddDefaultEventHandlers(logger, mwcInformer.Informer(), queue)
	controllerutils.AddDefaultEventHandlers(logger, vwcInformer.Informer(), queue)
	controllerutils.AddEventHandlers(
		secretInformer.Informer(),
		func(interface{}) { c.enqueueAll() },
		func(interface{}, interface{}) { c.enqueueAll() },
		func(interface{}) { c.enqueueAll() },
	)
	controllerutils.AddEventHandlers(
		configMapInformer.Informer(),
		func(interface{}) { c.enqueueAll() },
		func(interface{}, interface{}) { c.enqueueAll() },
		func(interface{}) { c.enqueueAll() },
	)
	controllerutils.AddEventHandlers(
		cpolInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks() },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks() },
		func(interface{}) { c.enqueueResourceWebhooks() },
	)
	controllerutils.AddEventHandlers(
		polInformer.Informer(),
		func(interface{}) { c.enqueueResourceWebhooks() },
		func(interface{}, interface{}) { c.enqueueResourceWebhooks() },
		func(interface{}) { c.enqueueResourceWebhooks() },
	)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	// add our known webhooks to the queue
	c.enqueueAll()
	controllerutils.Run(ctx, ControllerName, logger, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) enqueueAll() {
	c.enqueuePolicyWebhooks()
	c.enqueueResourceWebhooks()
	c.enqueueVerifyWebhook()
}

func (c *controller) enqueuePolicyWebhooks() {
	c.queue.Add(config.PolicyValidatingWebhookConfigurationName)
	c.queue.Add(config.PolicyMutatingWebhookConfigurationName)
}

func (c *controller) enqueueResourceWebhooks() {
	c.queue.Add(config.MutatingWebhookConfigurationName)
	c.queue.Add(config.ValidatingWebhookConfigurationName)
}

func (c *controller) enqueueVerifyWebhook() {
	c.queue.Add(config.VerifyMutatingWebhookConfigurationName)
}

func (c *controller) loadConfig() config.Configuration {
	cfg := config.NewDefaultConfiguration(nil)
	cm, err := c.configMapLister.ConfigMaps(config.KyvernoNamespace()).Get(config.KyvernoConfigMapName())
	if err == nil {
		cfg.Load(cm)
	}
	return cfg
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

func (c *controller) reconcileMutatingWebhookConfiguration(ctx context.Context, logger logr.Logger, name string) error {
	w, err := c.mwcLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	labels := w.GetLabels()
	if labels == nil || labels["webhook.kyverno.io/managed-by"] != kyvernov1.ValueKyvernoApp {
		return nil
	}
	cfg := c.loadConfig()
	webhookCfg := config.WebhookConfig{}
	webhookCfgs := cfg.GetWebhooks()
	if len(webhookCfgs) > 0 {
		webhookCfg = webhookCfgs[0]
	}
	caData, err := tls.ReadRootCASecret(c.secretClient)
	if err != nil {
		return err
	}
	_, err = controllerutils.Update(ctx, w, c.mwcClient, func(w *admissionregistrationv1.MutatingWebhookConfiguration) error {
		for i := range w.Webhooks {
			w.Webhooks[i].ClientConfig.CABundle = caData
			w.Webhooks[i].ObjectSelector = webhookCfg.ObjectSelector
			w.Webhooks[i].NamespaceSelector = webhookCfg.NamespaceSelector
		}
		return nil
	})
	return err
}

func (c *controller) reconcileValidatingWebhookConfiguration(ctx context.Context, logger logr.Logger, name string) error {
	w, err := c.vwcLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	labels := w.GetLabels()
	if labels == nil || labels["webhook.kyverno.io/managed-by"] != kyvernov1.ValueKyvernoApp {
		return nil
	}
	cfg := c.loadConfig()
	webhookCfg := config.WebhookConfig{}
	webhookCfgs := cfg.GetWebhooks()
	if len(webhookCfgs) > 0 {
		webhookCfg = webhookCfgs[0]
	}
	caData, err := tls.ReadRootCASecret(c.secretClient)
	if err != nil {
		return err
	}
	_, err = controllerutils.Update(ctx, w, c.vwcClient, func(w *admissionregistrationv1.ValidatingWebhookConfiguration) error {
		for i := range w.Webhooks {
			w.Webhooks[i].ClientConfig.CABundle = caData
			w.Webhooks[i].ObjectSelector = webhookCfg.ObjectSelector
			w.Webhooks[i].NamespaceSelector = webhookCfg.NamespaceSelector
		}
		return nil
	})
	return err
}

func (c *controller) reconcilePolicyValidatingWebhookConfiguration(ctx context.Context) error {
	return c.reconcileOneValidatingWebhookConfiguration(ctx, c.buildPolicyValidatingWebhookConfiguration)
}

func (c *controller) reconcilePolicyMutatingWebhookConfiguration(ctx context.Context) error {
	return c.reconcileOneMutatingWebhookConfiguration(ctx, c.buildPolicyMutatingWebhookConfiguration)
}

func (c *controller) reconcileVerifyMutatingWebhookConfiguration(ctx context.Context) error {
	return c.reconcileOneMutatingWebhookConfiguration(ctx, c.buildVerifyMutatingWebhookConfiguration)
}

func (c *controller) reconcileOneValidatingWebhookConfiguration(ctx context.Context, build func([]byte) *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	caData, err := tls.ReadRootCASecret(c.secretClient)
	if err != nil {
		return err
	}
	desired := build(caData)
	observed, err := c.vwcLister.Get(desired.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.vwcClient.Create(ctx, desired, metav1.CreateOptions{})
			return err
		}
		return err
	}
	_, err = controllerutils.Update(ctx, observed, c.vwcClient, func(w *admissionregistrationv1.ValidatingWebhookConfiguration) error {
		w.Labels = desired.Labels
		w.OwnerReferences = desired.OwnerReferences
		w.Webhooks = desired.Webhooks
		return nil
	})
	return err
}

func (c *controller) reconcileOneMutatingWebhookConfiguration(ctx context.Context, build func([]byte) *admissionregistrationv1.MutatingWebhookConfiguration) error {
	caData, err := tls.ReadRootCASecret(c.secretClient)
	if err != nil {
		return err
	}
	desired := build(caData)
	observed, err := c.mwcLister.Get(desired.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.mwcClient.Create(ctx, desired, metav1.CreateOptions{})
			return err
		}
		return err
	}
	_, err = controllerutils.Update(ctx, observed, c.mwcClient, func(w *admissionregistrationv1.MutatingWebhookConfiguration) error {
		w.Labels = desired.Labels
		w.OwnerReferences = desired.OwnerReferences
		w.Webhooks = desired.Webhooks
		return nil
	})
	return err
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	switch name {
	// case config.MutatingWebhookConfigurationName:
	// case config.ValidatingWebhookConfigurationName:
	case config.PolicyValidatingWebhookConfigurationName:
		return c.reconcilePolicyValidatingWebhookConfiguration(ctx)
	case config.PolicyMutatingWebhookConfigurationName:
		return c.reconcilePolicyMutatingWebhookConfiguration(ctx)
	case config.VerifyMutatingWebhookConfigurationName:
		return c.reconcileVerifyMutatingWebhookConfiguration(ctx)
	default:
		if err := c.reconcileMutatingWebhookConfiguration(ctx, logger, name); err != nil {
			return err
		}
		if err := c.reconcileValidatingWebhookConfiguration(ctx, logger, name); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) buildVerifyMutatingWebhookConfiguration(caBundle []byte) *admissionregistrationv1.MutatingWebhookConfiguration {
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
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": kyvernov1.ValueKyvernoApp,
				},
			},
		}},
	}
}

func (c *controller) buildPolicyMutatingWebhookConfiguration(caBundle []byte) *admissionregistrationv1.MutatingWebhookConfiguration {
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
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
		}},
	}
}

func (c *controller) buildPolicyValidatingWebhookConfiguration(caBundle []byte) *admissionregistrationv1.ValidatingWebhookConfiguration {
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
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
		}},
	}
}

func objectMeta(name string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			managedByLabel: kyvernov1.ValueKyvernoApp,
		},
		OwnerReferences: owner,
	}
}
