package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/utils"
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
	maxRetries = 10
)

var (
	none = admissionregistrationv1.SideEffectClassNone
	fail = admissionregistrationv1.Fail
	None = &none
	Fail = &fail
)

type controller struct {
	// clients
	vwcClient controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration]

	// listers
	vwcLister    admissionregistrationv1listers.ValidatingWebhookConfigurationLister
	secretLister corev1listers.SecretNamespaceLister

	// queue
	queue workqueue.RateLimitingInterface

	// config
	controllerName string
	logger         logr.Logger
	webhookName    string
	path           string
	server         string
	servicePort    int32
	rules          []admissionregistrationv1.RuleWithOperations
	failurePolicy  *admissionregistrationv1.FailurePolicyType
	sideEffects    *admissionregistrationv1.SideEffectClass
	configuration  config.Configuration
}

func NewController(
	controllerName string,
	vwcClient controllerutils.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
	secretInformer corev1informers.SecretInformer,
	webhookName string,
	path string,
	server string,
	servicePort int32,
	rules []admissionregistrationv1.RuleWithOperations,
	failurePolicy *admissionregistrationv1.FailurePolicyType,
	sideEffects *admissionregistrationv1.SideEffectClass,
	configuration config.Configuration,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)
	c := controller{
		vwcClient:      vwcClient,
		vwcLister:      vwcInformer.Lister(),
		secretLister:   secretInformer.Lister().Secrets(config.KyvernoNamespace()),
		queue:          queue,
		controllerName: controllerName,
		logger:         logging.ControllerLogger(controllerName),
		webhookName:    webhookName,
		path:           path,
		server:         server,
		servicePort:    servicePort,
		rules:          rules,
		failurePolicy:  failurePolicy,
		sideEffects:    sideEffects,
		configuration:  configuration,
	}
	controllerutils.AddDefaultEventHandlers(c.logger, vwcInformer.Informer(), queue)
	controllerutils.AddEventHandlersT(
		secretInformer.Informer(),
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == tls.GenerateRootCASecretName() {
				c.enqueue()
			}
		},
		func(_, obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == tls.GenerateRootCASecretName() {
				c.enqueue()
			}
		},
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == tls.GenerateRootCASecretName() {
				c.enqueue()
			}
		},
	)
	configuration.OnChanged(c.enqueue)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	c.enqueue()
	controllerutils.Run(ctx, c.logger, c.controllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) enqueue() {
	c.queue.Add(c.webhookName)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, _, _ string) error {
	if key != c.webhookName {
		return nil
	}
	caData, err := tls.ReadRootCASecret(c.secretLister)
	if err != nil {
		return err
	}
	desired, err := c.build(c.configuration, caData)
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
	_, err = controllerutils.Update(ctx, observed, c.vwcClient, func(w *admissionregistrationv1.ValidatingWebhookConfiguration) error {
		w.Labels = desired.Labels
		w.Annotations = desired.Annotations
		w.OwnerReferences = desired.OwnerReferences
		w.Webhooks = desired.Webhooks
		return nil
	})
	return err
}

func objectMeta(name string, annotations map[string]string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			utils.ManagedByLabel: kyvernov1.ValueKyvernoApp,
		},
		Annotations:     annotations,
		OwnerReferences: owner,
	}
}

func (c *controller) build(cfg config.Configuration, caBundle []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: objectMeta(c.webhookName, cfg.GetWebhookAnnotations()),
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:                    fmt.Sprintf("%s.%s.svc", config.KyvernoServiceName(), config.KyvernoNamespace()),
				ClientConfig:            c.clientConfig(caBundle),
				Rules:                   c.rules,
				FailurePolicy:           c.failurePolicy,
				SideEffects:             c.sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			}},
		},
		nil
}

func (c *controller) clientConfig(caBundle []byte) admissionregistrationv1.WebhookClientConfig {
	clientConfig := admissionregistrationv1.WebhookClientConfig{
		CABundle: caBundle,
	}
	if c.server == "" {
		clientConfig.Service = &admissionregistrationv1.ServiceReference{
			Namespace: config.KyvernoNamespace(),
			Name:      config.KyvernoServiceName(),
			Path:      &c.path,
			Port:      &c.servicePort,
		}
	} else {
		url := fmt.Sprintf("https://%s%s", c.server, c.path)
		clientConfig.URL = &url
	}
	return clientConfig
}
