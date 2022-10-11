package background

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/tls"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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
)

type controller struct {
	// clients
	secretClient controllerutils.GetClient[*corev1.Secret]
	mwcClient    controllerutils.UpdateClient[*admissionregistrationv1.MutatingWebhookConfiguration]
	vwcClient    controllerutils.UpdateClient[*admissionregistrationv1.ValidatingWebhookConfiguration]

	// listers
	secretLister    corev1listers.SecretLister
	configMapLister corev1listers.ConfigMapLister
	mwcLister       admissionregistrationv1listers.MutatingWebhookConfigurationLister
	vwcLister       admissionregistrationv1listers.ValidatingWebhookConfigurationLister

	// queue
	queue      workqueue.RateLimitingInterface
	mwcEnqueue controllerutils.EnqueueFunc
	vwcEnqueue controllerutils.EnqueueFunc
}

func NewController(
	secretClient controllerutils.GetClient[*corev1.Secret],
	mwcClient controllerutils.UpdateClient[*admissionregistrationv1.MutatingWebhookConfiguration],
	vwcClient controllerutils.UpdateClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
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
		mwcEnqueue:      controllerutils.AddDefaultEventHandlers(logger.V(3), mwcInformer.Informer(), queue),
		vwcEnqueue:      controllerutils.AddDefaultEventHandlers(logger.V(3), vwcInformer.Informer(), queue),
	}
	controllerutils.AddEventHandlersT(
		secretInformer.Informer(),
		func(obj *corev1.Secret) { c.secretChanged(obj) },
		func(_, obj *corev1.Secret) { c.secretChanged(obj) },
		func(obj *corev1.Secret) { c.secretChanged(obj) },
	)
	controllerutils.AddEventHandlersT(
		configMapInformer.Informer(),
		func(obj *corev1.ConfigMap) { c.configMapChanged(obj) },
		func(_, obj *corev1.ConfigMap) { c.configMapChanged(obj) },
		func(obj *corev1.ConfigMap) { c.configMapChanged(obj) },
	)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, ControllerName, logger.V(3), c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) secretChanged(secret *corev1.Secret) {
	if secret.GetName() == tls.GenerateRootCASecretName() && secret.GetNamespace() == config.KyvernoNamespace() {
		if err := c.enqueueAll(); err != nil {
			logger.Error(err, "failed to enqueue on secret change")
		}
	}
}

func (c *controller) configMapChanged(cm *corev1.ConfigMap) {
	if cm.GetName() == config.KyvernoConfigMapName() && cm.GetNamespace() == config.KyvernoNamespace() {
		if err := c.enqueueAll(); err != nil {
			logger.Error(err, "failed to enqueue on configmap change")
		}
	}
}

func (c *controller) enqueueAll() error {
	requirement, err := labels.NewRequirement("webhook.kyverno.io/managed-by", selection.Equals, []string{kyvernov1.ValueKyvernoApp})
	if err != nil {
		return err
	}
	selector := labels.Everything().Add(*requirement)
	mwcs, err := c.mwcLister.List(selector)
	if err != nil {
		return err
	}
	for _, mwc := range mwcs {
		err = c.mwcEnqueue(mwc)
		if err != nil {
			return err
		}
	}
	vwcs, err := c.vwcLister.List(selector)
	if err != nil {
		return err
	}
	for _, vwc := range vwcs {
		err = c.vwcEnqueue(vwc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) loadConfig() config.Configuration {
	cfg := config.NewDefaultConfiguration(nil)
	cm, err := c.configMapLister.ConfigMaps(config.KyvernoNamespace()).Get(config.KyvernoConfigMapName())
	if err == nil {
		cfg.Load(cm)
	}
	return cfg
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

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	if err := c.reconcileMutatingWebhookConfiguration(ctx, logger, name); err != nil {
		return err
	}
	if err := c.reconcileValidatingWebhookConfiguration(ctx, logger, name); err != nil {
		return err
	}
	return nil
}
