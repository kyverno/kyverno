package cleanup

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 5
)

type Controller struct {
	// typed client for kyverno CRDs
	kyvernoClient *kyvernoclient.Clientset
	// handler for GR CR
	syncHandler func(grKey string) error
	// handler to enqueue GR
	enqueueGR func(gr *kyverno.GenerateRequest)

	// control is used to delete the GR
	control ControlInterface
	// gr that need to be synced
	queue workqueue.RateLimitingInterface
	// pLister can list/get cluster policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister
	// pSynced returns true if the cluster policy has been synced at least once
	pSynced cache.InformerSynced
	// grSynced returns true if the generate request store has been synced at least once
	grSynced cache.InformerSynced
}

func NewController(
	kyvernoclient *kyvernoclient.Clientset,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
) *Controller {
	c := Controller{
		kyvernoClient: kyvernoclient,
		//TODO: do the math for worst case back off and make sure cleanup runs after that
		// as we dont want a deleted GR to be re-queue
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(1, 30), "generate-request-cleanup"),
	}
	c.control = Control{client: kyvernoclient}
	c.enqueueGR = c.enqueue
	c.syncHandler = c.syncGenerateRequest

	c.pLister = pInformer.Lister()
	c.grLister = grInformer.Lister().GenerateRequests("kyverno")

	c.pSynced = pInformer.Informer().HasSynced
	c.grSynced = grInformer.Informer().HasSynced

	pInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deletePolicy, // we only cleanup if the policy is delete
	}, 30)

	grInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addGR,
		UpdateFunc: c.updateGR,
		DeleteFunc: c.deleteGR,
	}, 30)

	return &c
}

func (c *Controller) deletePolicy(obj interface{}) {
	gr, ok := obj.(*kyverno.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		_, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
		if !ok {
			glog.Info(fmt.Errorf("Tombstone contained object that is not a Generate Request %#v", obj))
			return
		}
	}
	glog.V(4).Infof("Deleting Policy %s", gr.Name)
	// clean up the GR
	// Get the corresponding GR
}

func (c *Controller) addGR(obj interface{}) {
	gr := obj.(*kyverno.GenerateRequest)
	c.enqueueGR(gr)
}

func (c *Controller) updateGR(old, cur interface{}) {
	oldGr := old.(*kyverno.GenerateRequest)
	curGr := cur.(*kyverno.GenerateRequest)
	if oldGr.ResourceVersion == curGr.ResourceVersion {
		// Periodic resync will send update events for all known Namespace.
		// Two different versions of the same replica set will always have different RVs.
		return
	}
	c.enqueueGR(curGr)
}

func (c *Controller) deleteGR(obj interface{}) {
	gr, ok := obj.(*kyverno.GenerateRequest)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		_, ok = tombstone.Obj.(*kyverno.GenerateRequest)
		if !ok {
			glog.Info(fmt.Errorf("Tombstone contained object that is not a Generate Request %#v", obj))
			return
		}
	}
	glog.V(4).Infof("Deleting Policy %s", gr.Name)
	// sync Handler will remove it from the queue
	c.enqueueGR(gr)
}

func (c *Controller) enqueue(gr *kyverno.GenerateRequest) {
	key, err := cache.MetaNamespaceKeyFunc(gr)
	if err != nil {
		glog.Error(err)
		return
	}
	c.queue.Add(key)
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Info("Starting generate-policy-cleanup controller")
	defer glog.Info("Shutting down generate-policy-cleanup controller")

	if !cache.WaitForCacheSync(stopCh, c.pSynced, c.grSynced) {
		glog.Error("generate-policy-cleanup controller: failed to sync informer cache")
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	err := c.syncHandler(key.(string))
	c.handleErr(err, key)

	return true
}

func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		glog.Errorf("Error syncing Generate Request %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	utilruntime.HandleError(err)
	glog.Infof("Dropping generate request %q out of the queue: %v", key, err)
	c.queue.Forget(key)
}

func (c *Controller) syncGenerateRequest(key string) error {
	var err error
	startTime := time.Now()
	glog.V(4).Infof("Started syncing GR %q (%v)", key, startTime)
	defer func() {
		glog.V(4).Infof("Finished syncing GR %q (%v)", key, time.Since(startTime))
	}()
	gr, err := c.grLister.Get(key)
	if err != nil {
		return err
	}
	return c.processGR(*gr)
}
