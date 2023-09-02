package ttl

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerUtil "github.com/kyverno/kyverno/pkg/utils/controller"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	maxRetries   = 10
)

type controller struct {
	client       metadata.Getter
	queue        workqueue.RateLimitingInterface
	lister       cache.GenericLister
	wg           wait.Group
	queueFunc    controllerUtil.EnqueueFunc
	informer     cache.SharedIndexInformer
	registration cache.ResourceEventHandlerRegistration
	logger       logr.Logger
	metrics      ttlMetrics
	gvr          schema.GroupVersionResource
}

type ttlMetrics struct {
	deletedObjectsTotal metric.Int64Counter
	ttlFailureTotal     metric.Int64Counter
}

func keyFunc(obj metav1.Object) cache.ExplicitKey {
	return cache.ExplicitKey(obj.GetNamespace())
}

func newController(client metadata.Getter, metainformer informers.GenericInformer, logger logr.Logger, gvr schema.GroupVersionResource) (*controller, error) {
	c := &controller{
		client:   client,
		queue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		lister:   metainformer.Lister(),
		wg:       wait.Group{},
		informer: metainformer.Informer(),
		logger:   logger,
		metrics:  newTTLMetrics(logger),
	}
	// registration, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    c.handleAdd,
	// 	UpdateFunc: c.handleUpdate,
	// })
	// if err != nil {
	// 	logger.Error(err, "failed to register event handler")
	// 	return nil, err
	// }

	// controllerUtil.AddDelayedExplicitEventHandlers(logger, c.informer, c.queue, enqueueDelay, keyFunc)
	// enqueueFromTTL := func(obj metav1.Object) {
	// 	if controllerUtil.HasLabel(obj, kyverno.LabelCleanupTtl) {
	// 		c.queue.Add(keyFunc(obj))
	// 	}
	// }
	addFunc := controllerUtil.AddFunc(c.logger, c.queueFunc)
	updateFunc := controllerUtil.UpdateFunc(c.logger, c.queueFunc)
	registration, err := controllerUtil.AddEventHandlers(
		c.informer,
		addFunc,
		updateFunc,
		func(obj interface{}) {},
	)
	if err != nil {
		logger.Error(err, "failed to register even handlers")
		return nil, err
	}
	c.registration = registration
	c.queueFunc = controllerUtil.Queue(c.queue)
	return c, nil
}

func newTTLMetrics(logger logr.Logger) ttlMetrics {
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	deletedObjectsTotal, err := meter.Int64Counter(
		"kyverno_ttl_controller_deletedobjects",
		metric.WithDescription("can be used to track number of deleted objects by the ttl resource controller."),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, ttl_controller_deletedobjects_total")
	}
	ttlFailureTotal, err := meter.Int64Counter(
		"kyverno_ttl_controller_errors",
		metric.WithDescription("can be used to track number of ttl cleanup failures."),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, ttl_controller_errors_total")
	}
	return ttlMetrics{
		deletedObjectsTotal: deletedObjectsTotal,
		ttlFailureTotal:     ttlFailureTotal,
	}
}

// func (c *controller) handleAdd(obj interface{}) {
// 	controllerUtil.AddFunc(c.logger, c.queueFunc)
// }

// func (c *controller) handleUpdate(oldObj, newObj interface{}) {
// 	controllerUtil.UpdateFunc(c.logger, c.queueFunc)
// }

func (c *controller) Start(ctx context.Context, workers int) {
	// for i := 0; i < workers; i++ {
	// 	c.wg.StartWithContext(ctx, func(ctx context.Context) {
	// 		defer c.logger.V(3).Info("worker stopped")
	// 		c.logger.V(3).Info("worker starting ....")
	// 		wait.UntilWithContext(ctx, c.worker, 1*time.Second)
	// 	})
	// }

	controllerName := c.gvr.Group + c.gvr.Version + c.gvr.Resource

	controllerUtil.Run(ctx, c.logger, controllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) Stop() {
	defer c.logger.V(3).Info("queue stopped")
	defer c.wg.Wait()
	// Unregister the event handlers
	c.deregisterEventHandlers()
	c.logger.V(3).Info("queue stopping ....")
	c.queue.ShutDown()
}

func (c *controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Error(err, "failed to extract name")
		return
	}
	c.queue.Add(key)
}

// deregisterEventHandlers deregisters the event handlers from the informer.
func (c *controller) deregisterEventHandlers() {
	err := c.informer.RemoveEventHandler(c.registration)
	if err != nil {
		c.logger.Error(err, "failed to deregister event handlers")
		return
	}
	c.logger.V(3).Info("deregistered event handlers")
}

// func (c *controller) worker(ctx context.Context) {
// 	for {
// 		if !c.processItem() {
// 			// No more items in the queue, exit the loop
// 			break
// 		}
// 	}
// }

// func (c *controller) processItem() bool {
// 	item, shutdown := c.queue.Get()
// 	if shutdown {
// 		return false
// 	}
// 	// In any case we need to call Done
// 	defer c.queue.Done(item)
// 	logger := c.logger.WithValues("key", item.(string))
// 	err := c.reconcile(context.Background(), logger, item.(string),)
// 	if err != nil {
// 		c.logger.Error(err, "reconciliation failed")
// 		c.queue.AddRateLimited(item)
// 		return true
// 	} else {
// 		// If no error, we call Forget to reset the rate limiter
// 		c.queue.Forget(item)
// 	}
// 	return true
// }

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, itemKey string, _, _ string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(itemKey)
	if err != nil {
		return err
	}
	obj, err := c.lister.ByNamespace(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// resource doesn't exist anymore, nothing much to do at this point
			return nil
		}
		// there was an error, return it to requeue the key
		return err
	}

	metaObj, err := meta.Accessor(obj)
	if err != nil {
		logger.Info("object is not of type metav1.Object")
		return err
	}

	commonLabels := []attribute.KeyValue{
		attribute.String("resource_namespace", metaObj.GetNamespace()),
		attribute.String("resource_group", c.gvr.Group),
		attribute.String("resource_version", c.gvr.Version),
		attribute.String("resource", c.gvr.Resource),
	}

	// if the object is being deleted, return early
	if metaObj.GetDeletionTimestamp() != nil {
		return nil
	}

	labels := metaObj.GetLabels()
	ttlValue, ok := labels[kyverno.LabelCleanupTtl]

	if !ok {
		// No 'ttl' label present, no further action needed
		return nil
	}

	var deletionTime time.Time

	// Try parsing ttlValue as duration
	err = parseDeletionTime(metaObj, &deletionTime, ttlValue)

	if err != nil {
		logger.Error(err, "failed to parse label", "value", ttlValue)
		return nil
	}

	if time.Now().After(deletionTime) {
		err = c.client.Namespace(namespace).Delete(context.Background(), metaObj.GetName(), metav1.DeleteOptions{})
		if err != nil {
			logger.Error(err, "failed to delete resource")
			if c.metrics.ttlFailureTotal != nil {
				c.metrics.ttlFailureTotal.Add(context.Background(), 1, metric.WithAttributes(commonLabels...))
			}
			return err
		}
		logger.Info("resource has been deleted")
	} else {
		if c.metrics.deletedObjectsTotal != nil {
			c.metrics.deletedObjectsTotal.Add(context.Background(), 1, metric.WithAttributes(commonLabels...))
		}
		// Calculate the remaining time until deletion
		timeRemaining := time.Until(deletionTime)
		// Add the item back to the queue after the remaining time
		c.queue.AddAfter(itemKey, timeRemaining)
	}
	return nil
}
