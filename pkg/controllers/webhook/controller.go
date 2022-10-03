package background

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
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
	maxRetries = 10
	workers    = 2
)

type controller struct {
	// clients
	secretClient controllerutils.GetClient[*corev1.Secret]
	mwcClient    controllerutils.UpdateClient[*admissionregistrationv1.MutatingWebhookConfiguration]
	vwcClient    controllerutils.UpdateClient[*admissionregistrationv1.ValidatingWebhookConfiguration]

	// listers
	secretLister corev1listers.SecretLister
	mwcLister    admissionregistrationv1listers.MutatingWebhookConfigurationLister
	vwcLister    admissionregistrationv1listers.ValidatingWebhookConfigurationLister

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
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
) *controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)
	c := controller{
		secretClient: secretClient,
		mwcClient:    mwcClient,
		vwcClient:    vwcClient,
		secretLister: secretInformer.Lister(),
		mwcLister:    mwcInformer.Lister(),
		vwcLister:    vwcInformer.Lister(),
		queue:        queue,
		mwcEnqueue:   controllerutils.AddDefaultEventHandlers(logger.V(3), mwcInformer.Informer(), queue),
		vwcEnqueue:   controllerutils.AddDefaultEventHandlers(logger.V(3), vwcInformer.Informer(), queue),
	}
	controllerutils.AddEventHandlers(
		secretInformer.Informer(),
		func(obj interface{}) {
			if err := c.enqueue(obj.(*corev1.Secret)); err != nil {
				logger.Error(err, "failed to enqueue")
			}
		},
		func(_, obj interface{}) {
			if err := c.enqueue(obj.(*corev1.Secret)); err != nil {
				logger.Error(err, "failed to enqueue")
			}
		},
		func(obj interface{}) {
			if err := c.enqueue(obj.(*corev1.Secret)); err != nil {
				logger.Error(err, "failed to enqueue")
			}
		},
	)
	return &c
}

func (c *controller) Run(ctx context.Context) {
	controllerutils.Run(ctx, controllerName, logger.V(3), c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) enqueue(obj *corev1.Secret) error {
	if obj.GetName() == tls.GenerateRootCASecretName() && obj.GetNamespace() == config.KyvernoNamespace() {
		requirement, err := labels.NewRequirement("webhook.kyverno.io/managed-by", selection.Equals, []string{kyvernov1.ValueKyvernoApp})
		if err != nil {
			// TODO: log error
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
	}
	return nil
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
	_, err = controllerutils.Update(ctx, w, c.mwcClient, func(w *admissionregistrationv1.MutatingWebhookConfiguration) error {
		caData, err := tls.ReadRootCASecret(c.secretClient)
		if err != nil {
			return err
		}
		for i := range w.Webhooks {
			w.Webhooks[i].ClientConfig.CABundle = caData
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
	_, err = controllerutils.Update(ctx, w, c.vwcClient, func(w *admissionregistrationv1.ValidatingWebhookConfiguration) error {
		caData, err := tls.ReadRootCASecret(c.secretClient)
		if err != nil {
			return err
		}
		for i := range w.Webhooks {
			w.Webhooks[i].ClientConfig.CABundle = caData
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
