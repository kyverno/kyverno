package ttlcontroller

import (
	"context"
	"log"
	"time"

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
}

func newController(client metadata.Getter, metainformer informers.GenericInformer) *controller {
	c := &controller{
		client:   client,
		queue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		lister:   metainformer.Lister(),
		wg:       wait.Group{},
		informer: metainformer.Informer(),
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
	log.Println("resource was created")
	c.enqueue(obj)
}

func (c *controller) handleDelete(obj interface{}) {
	log.Println("resource was deleted")
	c.enqueue(obj)
}

func (c *controller) handleUpdate(oldObj, newObj interface{}) {
	log.Println("resource was updated")
	c.enqueue(newObj)
}

func (c *controller) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		c.wg.StartWithContext(ctx, func(ctx context.Context) {
			defer log.Println("worker stopped")
			log.Println("worker starting ....")
			wait.UntilWithContext(ctx, c.worker, 1*time.Second)
		})
	}
}

func (c *controller) Stop() {
	defer log.Println("queue stopped")
	defer c.wg.Wait()
	// Unregister the event handlers
	c.UnregisterEventHandlers()
	log.Println("queue stopping ....")
	c.queue.ShutDown()
}

func (c *controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("failed to extract name: %s", err)
		return
	}
	c.queue.Add(key)
}

// UnregisterEventHandlers unregisters the event handlers from the informer.
func (c *controller) UnregisterEventHandlers() {
	err := c.informer.RemoveEventHandler(c.eventRegistration)
	if err != nil {
		log.Printf("Unable to unregister event handlers: %s", err.Error())
		return
	}
	log.Println("unregister event handlers")
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
		log.Printf("reconciliation failed err: %s, for resource %s\n", err.Error(), item)
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
	// we now know the observed state, check against the desired state...
	// Assuming the object is of type metav1.Object, you can replace it with the correct type
	metaObj, error := meta.Accessor(obj)
	const ttlLabel = "kyverno.io/ttl"
	// fmt.Printf("the object is: %+v\n", metaObj)

	if error != nil {
		log.Printf("object '%s' is not of type metav1.Object", itemKey)
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
	ttlDuration, err := time.ParseDuration(ttlValue)
	if err == nil {
		creationTime := metaObj.GetCreationTimestamp().Time
		deletionTime = creationTime.Add(ttlDuration)
	} else {
		layoutRFCC := "2006-01-02T150405Z"
		// Try parsing ttlValue as a time in ISO 8601 format
		deletionTime, err = time.Parse(layoutRFCC, ttlValue)
		if err != nil {
			layoutCustom := "2006-01-02"
			deletionTime, err = time.Parse(layoutCustom, ttlValue)
			if err != nil {
				log.Printf("failed to parse TTL duration item %s ttlValue %s %+v", itemKey, ttlValue, err)
				return nil
			}
		}
	}

	log.Printf("the time to expire is: %s\n", deletionTime)

	if time.Now().After(deletionTime) {
		err = c.client.Namespace(namespace).Delete(context.Background(), metaObj.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Printf("failed to delete object: %s error: %+v", itemKey, err)
			return err
		}
		log.Printf("Resource '%s' has been deleted\n", itemKey)
	} else {
		// Calculate the remaining time until deletion
		timeRemaining := time.Until(deletionTime)
		// Add the item back to the queue after the remaining time
		c.queue.AddAfter(itemKey, timeRemaining)
	}
	return nil
}
