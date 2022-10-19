package certmanager

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/tls"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
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
	secretLister corev1listers.SecretLister

	// queue
	queue         workqueue.RateLimitingInterface
	secretEnqueue controllerutils.EnqueueFunc
}

func NewController(secretInformer corev1informers.SecretInformer, certRenewer tls.CertRenewer) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		renewer:       certRenewer,
		secretLister:  secretInformer.Lister(),
		queue:         queue,
		secretEnqueue: controllerutils.AddDefaultEventHandlers(logger, secretInformer.Informer(), queue),
	}
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	// we need to enqueue our secrets in case they don't exist yet in the cluster
	// this way we ensure the reconcile happens (hence renewal/creation)
	if err := c.secretEnqueue(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.KyvernoNamespace(),
			Name:      tls.GenerateTLSPairSecretName(),
		},
	}); err != nil {
		logger.Error(err, "failed to enqueue secret", "name", tls.GenerateTLSPairSecretName())
	}
	if err := c.secretEnqueue(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.KyvernoNamespace(),
			Name:      tls.GenerateRootCASecretName(),
		},
	}); err != nil {
		logger.Error(err, "failed to enqueue CA secret", "name", tls.GenerateRootCASecretName())
	}
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile, c.ticker)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	if namespace != config.KyvernoNamespace() {
		return nil
	}
	if name != tls.GenerateTLSPairSecretName() && name != tls.GenerateRootCASecretName() {
		return nil
	}
	return c.renewCertificates()
}

func (c *controller) ticker(ctx context.Context, logger logr.Logger) {
	certsRenewalTicker := time.NewTicker(tls.CertRenewalInterval)
	defer certsRenewalTicker.Stop()
	for {
		select {
		case <-certsRenewalTicker.C:
			list, err := c.secretLister.List(labels.Everything())
			if err == nil {
				for _, secret := range list {
					if err := c.secretEnqueue(secret); err != nil {
						logger.Error(err, "failed to enqueue secret", "name", secret.Name)
					}
				}
			} else {
				logger.Error(err, "falied to list secrets")
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *controller) renewCertificates() error {
	if err := common.RetryFunc(time.Second, 5*time.Second, c.renewer.RenewCA, "failed to renew CA", logger)(); err != nil {
		return err
	}
	if err := common.RetryFunc(time.Second, 5*time.Second, c.renewer.RenewTLS, "failed to renew TLS", logger)(); err != nil {
		return err
	}
	return nil
}
