package generate

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policyviolation"
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
	// event generator interface
	eventGen event.Interface
	// handler for GR CR
	syncHandler func(grKey string) error
	// handler to enqueue GR
	enqueueGR func(gr *kyverno.GenerateRequest)

	// grStatusControl is used to update GR status
	statusControl StatusControlInterface
	// Gr that need to be synced
	queue workqueue.RateLimitingInterface
	// pLister can list/get cluster policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister
	// pSynced returns true if the Cluster policy store has been synced at least once
	pSynced cache.InformerSynced
	// grSynced returns true if the Generate Request store has been synced at least once
	grSynced cache.InformerSynced
	// policy violation generator
	pvGenerator policyviolation.GeneratorInterface
	// dyanmic sharedinformer factory
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory
	//TODO: list of generic informers
	// only support Namespaces for re-evalutation on resource updates
	nsInformer informers.GenericInformer
}

func NewController(
	kyvernoclient *kyvernoclient.Clientset,
	client *dclient.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	eventGen event.Interface,
	pvGenerator policyviolation.GeneratorInterface,
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory,
) *Controller {
	c := Controller{
		client:        client,
		kyvernoClient: kyvernoclient,
		eventGen:      eventGen,
		pvGenerator:   pvGenerator,
		//TODO: do the math for worst case back off and make sure cleanup runs after that
		// as we dont want a deleted GR to be re-queue
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(1, 30), "generate-request"),
		dynamicInformer: dynamicInformer,
	}
	c.statusControl = StatusControl{client: kyvernoclient}

	pInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updatePolicy, // We only handle updates to policy
		// Deletion of policy will be handled by cleanup controller
	}, 2*time.Minute)

	grInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addGR,
		UpdateFunc: c.updateGR,
		DeleteFunc: c.deleteGR,
	}, 2*time.Minute)

	c.enqueueGR = c.enqueue
	c.syncHandler = c.syncGenerateRequest

	c.pLister = pInformer.Lister()
	c.grLister = grInformer.Lister().GenerateRequests("kyverno")

	c.pSynced = pInformer.Informer().HasSynced
	c.grSynced = pInformer.Informer().HasSynced

	//TODO: dynamic registration
	// Only supported for namespaces
	nsInformer := dynamicInformer.ForResource(client.DiscoveryClient.GetGVRFromKind("Namespace"))
	c.nsInformer = nsInformer
	c.nsInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updateGenericResource,
	}, 2*time.Minute)
	return &c
}

func (c *Controller) updateGenericResource(old, cur interface{}) {
	curR := cur.(*unstructured.Unstructured)

	grs, err := c.grLister.GetGenerateRequestsForResource(curR.GetKind(), curR.GetNamespace(), curR.GetName())
	if err != nil {
		glog.Errorf("failed to Generate Requests for resource %s/%s/%s: %v", curR.GetKind(), curR.GetNamespace(), curR.GetName(), err)
		return
	}
	// re-evaluate the GR as the resource was updated
	for _, gr := range grs {
		c.enqueueGR(gr)
	}

}

func (c *Controller) enqueue(gr *kyverno.GenerateRequest) {
	key, err := cache.MetaNamespaceKeyFunc(gr)
	if err != nil {
		glog.Error(err)
		return
	}
	c.queue.Add(key)
}

func (c *Controller) updatePolicy(old, cur interface{}) {
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)
	if oldP.ResourceVersion == curP.ResourceVersion {
		// Periodic resync will send update events for all known Namespace.
		// Two different versions of the same replica set will always have different RVs.
		return
	}
	glog.V(4).Infof("Updating Policy %s", oldP.Name)
	// get the list of GR for the current Policy version
	grs, err := c.grLister.GetGenerateRequestsForClusterPolicy(curP.Name)
	if err != nil {
		glog.Errorf("failed to Generate Requests for policy %s: %v", curP.Name, err)
		return
	}
	// re-evaluate the GR as the policy was updated
	for _, gr := range grs {
		c.enqueueGR(gr)
	}
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
	// only process the ones that are in "Pending"/"Completed" state
	// if the Generate Request fails due to incorrect policy, it will be requeued during policy update
	if curGr.Status.State == kyverno.Failed {
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
	glog.V(4).Infof("Deleting GR %s", gr.Name)
	// sync Handler will remove it from the queue
	c.enqueueGR(gr)
}

//Run ...
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Info("Starting generate-policy controller")
	defer glog.Info("Shutting down generate-policy controller")

	if !cache.WaitForCacheSync(stopCh, c.pSynced, c.grSynced) {
		glog.Error("generate-policy controller: failed to sync informer cache")
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
	if err != nil {
		return err
	}

	gr, err := c.grLister.Get(grName)
	if err != nil {
		glog.V(4).Info(err)
		return err
	}
	return c.processGR(gr)
}
