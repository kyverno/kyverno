package background

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/tls"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	managedByLabel = "webhook.kyverno.io/managed-by"
)

var (
	noneOnDryRun = admissionregistrationv1.SideEffectClassNoneOnDryRun
	ifNeeded     = admissionregistrationv1.IfNeededReinvocationPolicy
)

type controller struct {
	// clients
	secretClient controllerutils.GetClient[*corev1.Secret]
	mwcClient    controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration]
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

	// config
	server string
}

func NewController(
	secretClient controllerutils.GetClient[*corev1.Secret],
	mwcClient controllerutils.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration],
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
		mwcEnqueue:      controllerutils.AddDefaultEventHandlers(logger, mwcInformer.Informer(), queue),
		vwcEnqueue:      controllerutils.AddDefaultEventHandlers(logger, vwcInformer.Informer(), queue),
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
	// add our known webhooks to the queue
	c.queue.Add(config.MutatingWebhookConfigurationName)
	c.queue.Add(config.ValidatingWebhookConfigurationName)
	c.queue.Add(config.VerifyMutatingWebhookConfigurationName)
	c.queue.Add(config.PolicyValidatingWebhookConfigurationName)
	c.queue.Add(config.PolicyMutatingWebhookConfigurationName)
	controllerutils.Run(ctx, ControllerName, logger, c.queue, workers, maxRetries, c.reconcile)
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

func (c *controller) reconcileVerifyMutatingWebhookConfiguration(ctx context.Context) error {
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
	desired := c.buildVerifyMutatingWebhookConfiguration(caData, webhookCfg)
	observed, err := c.mwcLister.Get(desired.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.mwcClient.Create(ctx, desired, metav1.CreateOptions{})
			return err
		}
		return err
	}
	_, err = controllerutils.Update(ctx, observed, c.mwcClient, func(w *admissionregistrationv1.MutatingWebhookConfiguration) error {
		*w = *desired
		return nil
	})
	return err
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	switch name {
	// case config.MutatingWebhookConfigurationName:
	// case config.ValidatingWebhookConfigurationName:
	// case config.PolicyValidatingWebhookConfigurationName:
	// case config.PolicyMutatingWebhookConfigurationName:
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

func (c *controller) buildVerifyMutatingWebhookConfiguration(caBundle []byte, cfg config.WebhookConfig) *admissionregistrationv1.MutatingWebhookConfiguration {
	failurePolicy := admissionregistrationv1.Ignore
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: objectMeta(config.VerifyMutatingWebhookConfigurationName),
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name:         config.VerifyMutatingWebhookName,
			ClientConfig: c.clientConfig(caBundle, config.VerifyMutatingWebhookServicePath),
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule: admissionregistrationv1.Rule{
						Resources:   []string{"leases"},
						APIGroups:   []string{"coordination.k8s.io"},
						APIVersions: []string{"v1"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Update,
					},
				},
			},
			FailurePolicy:           &failurePolicy,
			SideEffects:             &noneOnDryRun,
			ReinvocationPolicy:      &ifNeeded,
			AdmissionReviewVersions: []string{"v1beta1"},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": kyvernov1.ValueKyvernoApp,
				},
			},
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
