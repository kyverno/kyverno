package cleanup

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 5
)

type Controller struct {
	// dyanmic client implementation
	client *dclient.Client
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
	// dyanmic sharedinformer factory
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory
	//TODO: list of generic informers
	// only support Namespaces for deletion of resource
	nsInformer informers.GenericInformer
}

func NewController(
	kyvernoclient *kyvernoclient.Clientset,
	client *dclient.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory,
) *Controller {
	c := Controller{
		kyvernoClient: kyvernoclient,
		client:        client,
		//TODO: do the math for worst case back off and make sure cleanup runs after that
		// as we dont want a deleted GR to be re-queue
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(1, 30), "generate-request-cleanup"),
		dynamicInformer: dynamicInformer,
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
	}, 2*time.Minute)

	grInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addGR,
		UpdateFunc: c.updateGR,
		DeleteFunc: c.deleteGR,
	}, 2*time.Minute)
	//TODO: dynamic registration
	// Only supported for namespaces
	nsInformer := dynamicInformer.ForResource(client.DiscoveryClient.GetGVRFromKind("Namespace"))
	c.nsInformer = nsInformer
	c.nsInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deleteGenericResource,
	}, 2*time.Minute)

	return &c
}

func (c *Controller) deleteGenericResource(obj interface{}) {
	r := obj.(*unstructured.Unstructured)
	grs, err := c.grLister.GetGenerateRequestsForResource(r.GetKind(), r.GetNamespace(), r.GetName())
	if err != nil {
		glog.Errorf("failed to Generate Requests for resource %s/%s/%s: %v", r.GetKind(), r.GetNamespace(), r.GetName(), err)
		return
	}
		// re-evaluate the GR as the resource was deleted
		for _, gr := range grs {
			c.enqueueGR(gr)
		}	
}

func (c *Controller) deletePolicy(obj interface{}) {
	p, ok := obj.(*kyverno.ClusterPolicy)
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
	glog.V(4).Infof("Deleting Policy %s", p.Name)
	// clean up the GR
	// Get the corresponding GR
	// get the list of GR for the current Policy version
	grs, err := c.grLister.GetGenerateRequestsForClusterPolicy(p.Name)
	if err != nil {
		glog.Errorf("failed to Generate Requests for policy %s: %v", p.Name, err)
		return
	}
	for _, gr := range grs {
		c.addGR(gr)
	}
}

func (c *Controller) addGR(obj interface{}) {
	gr := obj.(*kyverno.GenerateRequest)
	c.enqueueGR(gr)
}

func (c *Controller) updateGR(old, cur interface{}) {
	gr := cur.(*kyverno.GenerateRequest)
	c.enqueueGR(gr)
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
	glog.V(4).Infof("Deleting GR %s", gr.Name)
	// sync Handler will remove it from the queue
	c.enqueueGR(gr)
}

func (c *Controller) enqueue(gr *kyverno.GenerateRequest) {
	key, err := cache.MetaNamespaceKeyFunc(gr)
	if err != nil {
		glog.Error(err)
		return
	}
	glog.V(4).Infof("cleanup enqueu: %v", gr.Name)
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
	_, grName, err := cache.SplitMetaNamespaceKey(key)
	if errors.IsNotFound(err) {
		glog.Infof("Generate Request %s has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}
	gr, err := c.grLister.Get(grName)
	if err != nil {
		return err
	}
	return c.processGR(*gr)
}
