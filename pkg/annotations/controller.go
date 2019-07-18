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
	c.queue.Add(newInfo)
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

func (c *controller) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer c.queue.Done(obj)
		var key info
		var ok bool
		if key, ok = obj.(info); !ok {
			c.queue.Forget(obj)
			glog.Warningf("Expecting type info by got %v\n", obj)
			return nil
		}
		// Run the syncHandler, passing the resource and the policy
		if err := c.SyncHandler(key); err != nil {
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s/%s/%s' : %s, requeuing annotation creation request", key.RKind, key.RNs, key.RName, err)
		}
		return nil
	}(obj)

	if err != nil {
		glog.Warning(err)
	}
	return true
}

func (c *controller) SyncHandler(key info) error {
	var err error
	// check if the resource is created
	_, err = c.client.GetResource(key.RKind, key.RNs, key.RName)
	if err != nil {
		glog.Errorf("Error creating annotation: unable to get resource %s/%s/%s, will retry: %s ", key.RKind, key.RNs, key.RName, err)
		return err
	}
	// if it is patch the resource
	_, err = c.client.PatchResource(key.RKind, key.RNs, key.RName, key.Patch)
	if err != nil {
		glog.Errorf("Error creating annotation: unable to get resource %s/%s/%s, will retry: %s", key.RKind, key.RNs, key.RName, err)
		return err
	}
	return nil
}
