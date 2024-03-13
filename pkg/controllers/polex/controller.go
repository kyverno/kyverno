package polex

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernov2beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kyvernov2beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/polex/store"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "polex"
	maxRetries     = 10
)

type controller struct {
	// listers
	polexLister kyvernov2beta1listers.PolicyExceptionLister

	// queue
	queue workqueue.RateLimitingInterface

	// state
	namespace string
	store     store.Store
}

func NewController(
	polexInformer kyvernov2beta1informers.PolicyExceptionInformer,
	namespace string,
	storage store.Store,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := &controller{
		polexLister: polexInformer.Lister(),
		queue:       queue,
		namespace:   namespace,
		store:       storage,
	}

	if _, err := controllerutils.AddEventHandlersT(polexInformer.Informer(), c.addPolex, c.updatePolex, c.deletePolex); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	return c
}

func (c *controller) addPolex(obj *kyvernov2beta1.PolicyException) {
	logger.Info("polex created", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	if c.namespace == "" || obj.Namespace == c.namespace {
		c.enqueuePolex(obj)
	}
}

func (c *controller) updatePolex(old, obj *kyvernov2beta1.PolicyException) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	logger.Info("polex updated", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	if c.namespace == "" || obj.Namespace == c.namespace {
		c.enqueuePolex(obj)
	}
}

func (c *controller) deletePolex(obj *kyvernov2beta1.PolicyException) {
	if c.namespace == "" || obj.Namespace == c.namespace {
		c.store.Delete(obj)
	}
}

func (c *controller) enqueuePolex(polex *kyvernov2beta1.PolicyException) {
	key, err := cache.MetaNamespaceKeyFunc(polex)
	if err != nil {
		logger.Error(err, "failed to enqueue polex")
		return
	}
	c.queue.Add(key)
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	polex, err := c.getEntry(namespace, name)
	if err != nil {
		return err
	}

	c.store.Delete(polex)
	c.store.Add(polex)
	return nil
}

func (c *controller) getEntry(namespace, name string) (*kyvernov2beta1.PolicyException, error) {
	return c.polexLister.PolicyExceptions(namespace).Get(name)
}
