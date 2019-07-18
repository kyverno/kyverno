package annotations

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	client "github.com/nirmata/kyverno/pkg/dclient"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	client *client.Client
	queue  workqueue.RateLimitingInterface
}

type Interface interface {
	Add(rkind, rns, rname string, patch []byte)
}

type Controller interface {
	Interface
	Run(stopCh <-chan struct{})
	Stop()
}

func NewAnnotationControler(client *client.Client) Controller {
	return &controller{
		client: client,
		queue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), annotationQueueName),
	}
}

func (c *controller) Add(rkind, rns, rname string, patch []byte) {
	c.queue.Add(newInfo(rkind, rns, rname, patch))
}

func (c *controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	for i := 0; i < workerThreadCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	glog.Info("Started annotation controller workers")
}

func (c *controller) Stop() {
	defer c.queue.ShutDown()
	glog.Info("Shutting down annotation controller workers")
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (pc *controller) processNextWorkItem() bool {
	obj, shutdown := pc.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer pc.queue.Done(obj)
		err := pc.syncHandler(obj)
		pc.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		glog.Error(err)
		return true
	}
	return true
}
func (pc *controller) handleErr(err error, key interface{}) {
	if err == nil {
		pc.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if pc.queue.NumRequeues(key) < WorkQueueRetryLimit {
		glog.Warningf("Error syncing events %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		pc.queue.AddRateLimited(key)
		return
	}
	pc.queue.Forget(key)
	glog.Error(err)
	glog.Warningf("Dropping the key %q out of the queue: %v", key, err)
}

func (c *controller) syncHandler(obj interface{}) error {
	var key info
	var ok bool
	if key, ok = obj.(info); !ok {
		return fmt.Errorf("expected string in workqueue but got %#v", obj)
	}

	var err error
	// check if the resource is created
	_, err = c.client.GetResource(key.RKind, key.RNs, key.RName)
	if err != nil {
		glog.Errorf("Error creating annotation: unable to get resource %s/%s/%s, will retry: %s ", key.RKind, key.RNs, key.RName, err)
		return err
	}
	// if it is patch the resource
	_, err = c.client.PatchResource(key.RKind, key.RNs, key.RName, *key.Patch)
	if err != nil {
		glog.Errorf("Error creating annotation: unable to get resource %s/%s/%s, will retry: %s", key.RKind, key.RNs, key.RName, err)
		return err
	}
	return nil
}
