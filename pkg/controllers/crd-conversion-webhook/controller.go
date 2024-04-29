package conversionwebhook

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/tls"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "crd-conversion-webhook"
	maxRetries     = 10
)

type controller struct {
	// clients
	apiserverClient apiserver.Interface

	// listers
	secretLister corev1listers.SecretNamespaceLister

	// queue
	queue workqueue.RateLimitingInterface

	// config
	caSecretName string
	logger       logr.Logger
	server       string
	servicePort  int32
	path         string
}

func NewController(
	apiserverClient apiserver.Interface,
	secretInformer corev1informers.SecretInformer,
	caSecretName string,
	server string,
	servicePort int32,
	path string,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		apiserverClient: apiserverClient,
		secretLister:    secretInformer.Lister().Secrets(config.KyvernoNamespace()),
		queue:           queue,
		caSecretName:    caSecretName,
		server:          server,
		servicePort:     servicePort,
		path:            path,
		logger:          logging.ControllerLogger(ControllerName),
	}
	if _, err := controllerutils.AddEventHandlersT(
		secretInformer.Informer(),
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == caSecretName {
				c.enqueue()
			}
		},
		func(_, obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == caSecretName {
				c.enqueue()
			}
		},
		func(obj *corev1.Secret) {
			if obj.GetNamespace() == config.KyvernoNamespace() && obj.GetName() == caSecretName {
				c.enqueue()
			}
		},
	); err != nil {
		c.logger.Error(err, "failed to register event handlers")
	}
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	c.enqueue()
	controllerutils.Run(ctx, c.logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) enqueue() {
	c.queue.Add("policies.kyverno.io")
	c.queue.Add("clusterpolicies.kyverno.io")
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	caData, err := tls.ReadRootCASecret(c.caSecretName, config.KyvernoNamespace(), c.secretLister)
	if err != nil {
		return err
	}
	conversionwebhook, err := c.buildCustomResourceConversion(caData)
	if err != nil {
		return err
	}

	policy, err := c.apiserverClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "policies.kyverno.io", metav1.GetOptions{})
	if err != nil {
		return err
	}

	_, err = controllerutils.Update(ctx, policy, c.apiserverClient.ApiextensionsV1().CustomResourceDefinitions(), func(crd *apiextensionsv1.CustomResourceDefinition) error {
		crd.Spec.Conversion = conversionwebhook
		return nil
	})
	if err != nil {
		return err
	}

	clusterpolicy, err := c.apiserverClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "clusterpolicies.kyverno.io", metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, err = controllerutils.Update(ctx, clusterpolicy, c.apiserverClient.ApiextensionsV1().CustomResourceDefinitions(), func(crd *apiextensionsv1.CustomResourceDefinition) error {
		crd.Spec.Conversion = conversionwebhook
		return nil
	})
	return nil
}

func (c *controller) buildCustomResourceConversion(caBundle []byte) (*apiextensionsv1.CustomResourceConversion, error) {
	return &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig:             c.clientConfig(caBundle),
			ConversionReviewVersions: []string{"v1", "v2beta1", "v2"},
		},
	}, nil
}

func (c *controller) clientConfig(caBundle []byte) *apiextensionsv1.WebhookClientConfig {
	clientConfig := apiextensionsv1.WebhookClientConfig{
		CABundle: caBundle,
	}
	if c.server == "" {
		clientConfig.Service = &apiextensionsv1.ServiceReference{
			Namespace: config.KyvernoNamespace(),
			Name:      config.KyvernoServiceName(),
			Path:      &c.path,
			Port:      &c.servicePort,
		}
	} else {
		url := fmt.Sprintf("https://%s%s", c.server, c.path)
		clientConfig.URL = &url
	}
	return &clientConfig
}
