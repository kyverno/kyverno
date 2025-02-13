package ttl

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

const (
	// Workers is the number of workers for this controller
	maxRetries = 10
)

type controller struct {
	name         string
	client       metadata.Getter
	queue        workqueue.TypedRateLimitingInterface[any]
	lister       cache.GenericLister
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

func newController(client metadata.Getter, metainformer informers.GenericInformer, logger logr.Logger, gvr schema.GroupVersionResource) (*controller, error) {
	name := gvr.Version + "/" + gvr.Resource
	if gvr.Group != "" {
		name = gvr.Group + "/" + name
	}
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[any](), workqueue.TypedRateLimitingQueueConfig[any]{Name: name})
	c := &controller{
		name:     name,
		client:   client,
		queue:    queue,
		lister:   metainformer.Lister(),
		informer: metainformer.Informer(),
		logger:   logger,
		metrics:  newTTLMetrics(logger),
		gvr:      gvr,
	}
	enqueue := controllerutils.LogError(logger, controllerutils.Parse(controllerutils.MetaNamespaceKey, controllerutils.Queue(queue)))
	registration, err := controllerutils.AddEventHandlers(
		c.informer,
		controllerutils.AddFunc(logger, enqueue),
		controllerutils.UpdateFunc(logger, enqueue),
		nil,
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers")
		return nil, err
	}
	c.registration = registration
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

func (c *controller) Start(ctx context.Context, workers int) {
	controllerutils.Run(ctx, c.logger, c.name, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) Stop() {
	defer c.logger.V(3).Info("queue stopped")
	// Unregister the event handlers
	c.deregisterEventHandlers()
	c.logger.V(3).Info("queue stopping ....")
	c.queue.ShutDown()
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

// Function to determine the deletion propagation policy
func determinePropagationPolicy(metaObj metav1.Object, logger logr.Logger) *metav1.DeletionPropagation {
	annotations := metaObj.GetAnnotations()
	if annotations == nil {
		return nil
	}
	switch annotations[kyverno.AnnotationCleanupPropagationPolicy] {
	case "Foreground":
		return ptr.To(metav1.DeletePropagationForeground)
	case "Background":
		return ptr.To(metav1.DeletePropagationBackground)
	case "Orphan":
		return ptr.To(metav1.DeletePropagationOrphan)
	case "":
		return nil
	default:
		logger.V(2).Info("Unknown propagationPolicy annotation, no global policy found", "policy", annotations[kyverno.AnnotationCleanupPropagationPolicy])
		return nil
	}
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, itemKey string, _, _ string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(itemKey)
	if err != nil {
		return err
	}
	getter := c.lister.Get
	if namespace != "" {
		getter = c.lister.ByNamespace(namespace).Get
	}
	obj, err := getter(name)
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
		logger.V(2).Info("object is not of type metav1.Object")
		return err
	}
	commonLabels := []attribute.KeyValue{
		attribute.String("resource_namespace", metaObj.GetNamespace()),
		attribute.String("resource_group", c.gvr.Group),
		attribute.String("resource_version", c.gvr.Version),
		attribute.String("resource_resource", c.gvr.Resource),
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
	if err := parseDeletionTime(metaObj, &deletionTime, ttlValue); err != nil {
		logger.Error(err, "failed to parse label", "value", ttlValue)
		return nil
	}
	if time.Now().After(deletionTime) {
		deleteOptions := metav1.DeleteOptions{
			PropagationPolicy: determinePropagationPolicy(metaObj, logger),
		}
		err = c.client.Namespace(namespace).Delete(context.Background(), metaObj.GetName(), deleteOptions)
		if err != nil {
			logger.Error(err, "failed to delete resource")
			if c.metrics.ttlFailureTotal != nil {
				c.metrics.ttlFailureTotal.Add(context.Background(), 1, metric.WithAttributes(commonLabels...))
			}
			return err
		}
		logger.V(2).Info("resource has been deleted")
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
