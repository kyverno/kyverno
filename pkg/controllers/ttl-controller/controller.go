package ttlcontroller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	client       metadata.Getter
	queue        workqueue.RateLimitingInterface
	lister       cache.GenericLister
	wg           wait.Group
	informer     cache.SharedIndexInformer
	registration cache.ResourceEventHandlerRegistration
	logger       logr.Logger
}

func newController(client metadata.Getter, metainformer informers.GenericInformer, logger logr.Logger) (*controller, error) {
	c := &controller{
		client:   client,
		queue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		lister:   metainformer.Lister(),
		wg:       wait.Group{},
		informer: metainformer.Informer(),
		logger:   logger,
	}
	registration, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleAdd,
		DeleteFunc: c.handleDelete,
		UpdateFunc: c.handleUpdate,
	})
	if err != nil {
		logger.Error(err, "failed to register event handler")
		return nil, err
	}
	c.registration = registration
	return c, nil
}

func (c *controller) handleAdd(obj interface{}) {
	c.enqueue(obj)
}

func (c *controller) handleDelete(obj interface{}) {
	c.enqueue(obj)
}

func (c *controller) handleUpdate(oldObj, newObj interface{}) {
	c.enqueue(newObj)
}

func (c *controller) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		c.wg.StartWithContext(ctx, func(ctx context.Context) {
			defer c.logger.Info("worker stopped")
			c.logger.Info("worker starting ....")
			wait.UntilWithContext(ctx, c.worker, 1*time.Second)
		})
	}
}

func (c *controller) Stop() {
	defer c.logger.Info("queue stopped")
	defer c.wg.Wait()
	// Unregister the event handlers
	c.deregisterEventHandlers()
	c.logger.Info("queue stopping ....")
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
	c.logger.Info("deregistered event handlers")
}

func (c *controller) worker(ctx context.Context) {
	for {
		if !c.processItem() {
			// No more items in the queue, exit the loop
			break
		}
	}
}

func (c *controller) processItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Forget(item)
	err := c.reconcile(item.(string))
	if err != nil {
		c.logger.Error(err, "reconciliation failed")
		c.queue.AddRateLimited(item)
		return true
	}
	c.queue.Done(item)
	return true
}

func (c *controller) reconcile(itemKey string) error {
	logger := c.logger.WithValues("key", itemKey)
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
			return err
		}
		logger.Info("resource has been deleted")
	} else {
		// Calculate the remaining time until deletion
		timeRemaining := time.Until(deletionTime)
		// Add the item back to the queue after the remaining time
		c.queue.AddAfter(itemKey, timeRemaining)
	}
	return nil
}
