package ttlcontroller

import (
	"context"
	"log"
	"time"

	"github.com/go-logr/logr"
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
	client            metadata.Getter
	queue             workqueue.RateLimitingInterface
	lister            cache.GenericLister
	wg                wait.Group
	informer          cache.SharedIndexInformer
	eventRegistration cache.ResourceEventHandlerRegistration
	controllerLogger  logr.Logger
}

func newController(client metadata.Getter, metainformer informers.GenericInformer, logger logr.Logger) *controller {
	c := &controller{
		client:           client,
		queue:            workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		lister:           metainformer.Lister(),
		wg:               wait.Group{},
		informer:         metainformer.Informer(),
		controllerLogger: logger,
	}

	eventRegistration, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleAdd,
		DeleteFunc: c.handleDelete,
		UpdateFunc: c.handleUpdate,
	})
	if err != nil {
		log.Printf("error in registering event handler: %v", err)
	}

	c.eventRegistration = eventRegistration

	return c
}

func (c *controller) handleAdd(obj interface{}) {
	c.controllerLogger.Info("resource was created")
	c.enqueue(obj)
}

func (c *controller) handleDelete(obj interface{}) {
	c.enqueue(obj)
}

func (c *controller) handleUpdate(oldObj, newObj interface{}) {
	c.controllerLogger.Info("resource was updated")
	c.enqueue(newObj)
}

func (c *controller) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		c.wg.StartWithContext(ctx, func(ctx context.Context) {
			defer c.controllerLogger.Info("worker stopped")
			c.controllerLogger.Info("worker starting ....")
			wait.UntilWithContext(ctx, c.worker, 1*time.Second)
		})
	}
}

func (c *controller) Stop() {
	defer c.controllerLogger.Info("queue stopped")
	defer c.wg.Wait()
	// Unregister the event handlers
	c.DeregisterEventHandlers()
	c.controllerLogger.Info("queue stopping ....")
	c.queue.ShutDown()
}

func (c *controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.controllerLogger.Error(err, "failed to extract name")
		return
	}
	c.queue.Add(key)
}

// DeregisterEventHandlers deregisters the event handlers from the informer.
func (c *controller) DeregisterEventHandlers() {
	err := c.informer.RemoveEventHandler(c.eventRegistration)
	if err != nil {
		c.controllerLogger.Error(err, "Unable to deregister event handlers")
		return
	}
	c.controllerLogger.Info("deregister event handlers")
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

	log.Printf("%+v\n", item)
	defer c.queue.Forget(item)
	err := c.reconcile(item.(string))
	if err != nil {
		c.controllerLogger.Error(err, "reconciliation failed for resource %s\n", item)
		c.queue.AddRateLimited(item)
		return true
	}
	c.queue.Done(item)
	return true
}

func (c *controller) reconcile(itemKey string) error {
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

	metaObj, error := meta.Accessor(obj)
	const ttlLabel = "kyverno.io/ttl"

	if error != nil {
		c.controllerLogger.Info("object '%s' is not of type metav1.Object", itemKey)
		return err
	}

	labels := metaObj.GetLabels()
	ttlValue, ok := labels[ttlLabel]

	if !ok {
		// No 'ttl' label present, no further action needed
		return nil
	}

	var deletionTime time.Time

	// Try parsing ttlValue as duration
	err = parseDeletionTime(metaObj, &deletionTime, ttlValue)

	if err != nil {
		c.controllerLogger.Error(err, "failed to parse TTL duration item %s ttlValue %s", itemKey, ttlValue)
		return nil
	}

	c.controllerLogger.Info("the time to expire is: ", deletionTime.String())

	if time.Now().After(deletionTime) {
		err = c.client.Namespace(namespace).Delete(context.Background(), metaObj.GetName(), metav1.DeleteOptions{})
		if err != nil {
			c.controllerLogger.Error(err, "failed to delete object: %s", itemKey)
			return err
		}
		// log.Printf("Resource '%s' has been deleted\n", itemKey)
		c.controllerLogger.Info("Resource", itemKey, " has been deleted")
	} else {
		// Calculate the remaining time until deletion
		timeRemaining := time.Until(deletionTime)
		// Add the item back to the queue after the remaining time
		c.queue.AddAfter(itemKey, timeRemaining)
	}
	return nil
}
