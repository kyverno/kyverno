package configmap

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/informers"
	"github.com/kyverno/kyverno/pkg/logging"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
)

type Controller interface {
	controllers.Controller
	WarmUp(context.Context) error
}

type controller struct {
	// listers
	informer cache.SharedIndexInformer
	lister   corev1listers.ConfigMapNamespaceLister

	// queue
	queue workqueue.RateLimitingInterface

	// config
	controllerName  string
	logger          logr.Logger
	name            string
	callback        callback
	resourceVersion string
}

type callback func(context.Context, *corev1.ConfigMap) error

func NewController(
	controllerName string,
	client kubernetes.Interface,
	resyncPeriod time.Duration,
	namespace string,
	name string,
	callback callback,
) Controller {
	informer := informers.NewConfigMapInformer(client, namespace, name, resyncPeriod)
	c := controller{
		informer:       informer.Informer(),
		lister:         informer.Lister().ConfigMaps(namespace),
		controllerName: controllerName,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		logger:         logging.ControllerLogger(controllerName),
		name:           name,
		callback:       callback,
	}
	if _, _, err := controllerutils.AddDefaultEventHandlers(c.logger, informer.Informer(), c.queue); err != nil {
		logging.Error(err, "failed to register event handlers")
	}
	return &c
}

func (c *controller) WarmUp(ctx context.Context) error {
	go c.informer.Run(ctx.Done())
	if synced := cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced); !synced {
		return errors.New("configmap informer cache failed to sync")
	}
	return c.doReconcile(ctx, c.logger)
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, c.logger, c.controllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, _, _, _ string) error {
	return c.doReconcile(ctx, c.logger)
}

func (c *controller) doReconcile(ctx context.Context, logger logr.Logger) error {
	observed, err := c.lister.Get(c.name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return c.callback(ctx, nil)
	}
	if c.resourceVersion == observed.ResourceVersion {
		return nil
	}
	if err := c.callback(ctx, observed); err != nil {
		return err
	}
	// record resource version
	c.resourceVersion = observed.ResourceVersion
	return nil
}
