package certmanager

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/tls"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	retryutils "github.com/kyverno/kyverno/pkg/utils/retry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "certmanager-controller"
	maxRetries     = 10
)

type controller struct {
	renewer tls.CertRenewer

	// listers
	caLister  corev1listers.SecretLister
	tlsLister corev1listers.SecretLister

	// queue
	queue      workqueue.RateLimitingInterface
	caEnqueue  controllerutils.EnqueueFunc
	tlsEnqueue controllerutils.EnqueueFunc
}

func NewController(
	caInformer corev1informers.SecretInformer,
	tlsInformer corev1informers.SecretInformer,
	certRenewer tls.CertRenewer,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		renewer:    certRenewer,
		caLister:   caInformer.Lister(),
		tlsLister:  tlsInformer.Lister(),
		queue:      queue,
		caEnqueue:  controllerutils.AddDefaultEventHandlers(logger, caInformer.Informer(), queue),
		tlsEnqueue: controllerutils.AddDefaultEventHandlers(logger, tlsInformer.Informer(), queue),
	}
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	// we need to enqueue our secrets in case they don't exist yet in the cluster
	// this way we ensure the reconcile happens (hence renewal/creation)
	if err := c.tlsEnqueue(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.KyvernoNamespace(),
			Name:      config.GenerateTLSPairSecretName(),
		},
	}); err != nil {
		logger.Error(err, "failed to enqueue secret", "name", config.GenerateTLSPairSecretName())
	}
	if err := c.caEnqueue(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.KyvernoNamespace(),
			Name:      config.GenerateRootCASecretName(),
		},
	}); err != nil {
		logger.Error(err, "failed to enqueue CA secret", "name", config.GenerateRootCASecretName())
	}
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile, c.ticker)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	if namespace != config.KyvernoNamespace() {
		return nil
	}
	if name != config.GenerateTLSPairSecretName() && name != config.GenerateRootCASecretName() {
		return nil
	}
	return c.renewCertificates(ctx)
}

func (c *controller) ticker(ctx context.Context, logger logr.Logger) {
	certsRenewalTicker := time.NewTicker(tls.CertRenewalInterval)
	defer certsRenewalTicker.Stop()
	for {
		select {
		case <-certsRenewalTicker.C:
			{
				list, err := c.caLister.List(labels.Everything())
				if err == nil {
					for _, secret := range list {
						if err := c.caEnqueue(secret); err != nil {
							logger.Error(err, "failed to enqueue secret", "name", secret.Name)
						}
					}
				} else {
					logger.Error(err, "falied to list secrets")
				}
			}
			{
				list, err := c.tlsLister.List(labels.Everything())
				if err == nil {
					for _, secret := range list {
						if err := c.tlsEnqueue(secret); err != nil {
							logger.Error(err, "failed to enqueue secret", "name", secret.Name)
						}
					}
				} else {
					logger.Error(err, "falied to list secrets")
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *controller) renewCertificates(ctx context.Context) error {
	if err := retryutils.RetryFunc(ctx, time.Second, 5*time.Second, logger, "failed to renew CA", c.renewer.RenewCA)(); err != nil {
		return err
	}
	if err := retryutils.RetryFunc(ctx, time.Second, 5*time.Second, logger, "failed to renew TLS", c.renewer.RenewTLS)(); err != nil {
		return err
	}
	return nil
}
