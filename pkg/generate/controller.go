package generate

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	"github.com/nirmata/kyverno/pkg/constant"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policystatus"
	"github.com/nirmata/kyverno/pkg/resourcecache"
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

// Controller manages the life-cycle for Generate-Requests and applies generate rule
type Controller struct {
	ctx context.Context
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
	// dyanmic sharedinformer factory
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory
	//TODO: list of generic informers
	// only support Namespaces for re-evalutation on resource updates
	nsInformer           informers.GenericInformer
	policyStatusListener policystatus.Listener
	log                  logr.Logger

	Config   config.Interface
	resCache resourcecache.ResourceCacheIface
}

//NewController returns an instance of the Generate-Request Controller
func NewController(
	ctx context.Context,
	kyvernoclient *kyvernoclient.Clientset,
	client *dclient.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	eventGen event.Interface,
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory,
	policyStatus policystatus.Listener,
	log logr.Logger,
	dynamicConfig config.Interface,
	resCache resourcecache.ResourceCacheIface,
) *Controller {
	c := Controller{
		ctx:           ctx,
		client:        client,
		kyvernoClient: kyvernoclient,
		eventGen:      eventGen,
		//TODO: do the math for worst case back off and make sure cleanup runs after that
		// as we dont want a deleted GR to be re-queue
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(1, 30), "generate-request"),
		dynamicInformer:      dynamicInformer,
		log:                  log,
		policyStatusListener: policyStatus,
		Config:               dynamicConfig,
		resCache:             resCache,
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
	c.grLister = grInformer.Lister().GenerateRequests(config.KubePolicyNamespace)

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
	logger := c.log
	curR := cur.(*unstructured.Unstructured)

	grs, err := c.grLister.GetGenerateRequestsForResource(curR.GetKind(), curR.GetNamespace(), curR.GetName())
	if err != nil {
		logger.Error(err, "failed to get generate request CR for the resoource", "kind", curR.GetKind(), "name", curR.GetName(), "namespace", curR.GetNamespace())
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
		c.log.Error(err, "failed to extract name")
		return
	}
	c.queue.Add(key)
}

func (c *Controller) updatePolicy(old, cur interface{}) {
	logger := c.log
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)
	if oldP.ResourceVersion == curP.ResourceVersion {
		// Periodic resync will send update events for all known Namespace.
		// Two different versions of the same replica set will always have different RVs.
		return
	}
	logger.V(4).Info("updating policy", "name", oldP.Name)
	// get the list of GR for the current Policy version
	grs, err := c.grLister.GetGenerateRequestsForClusterPolicy(curP.Name)
	if err != nil {
		logger.Error(err, "failed to generate request for policy", "name", curP.Name)
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
	logger := c.log
	gr, ok := obj.(*kyverno.GenerateRequest)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("Couldn't get object from tombstone", "obj", obj)
			return
		}
		_, ok = tombstone.Obj.(*kyverno.GenerateRequest)
		if !ok {
			logger.Info("tombstone contained object that is not a Generate Request CR", "obj", obj)
			return
		}
	}
	for _, resource := range gr.Status.GeneratedResources {
		r, err := c.client.GetResource(resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
		if err != nil {
			logger.Error(err, "Generated resource is not deleted", "Resource", resource.Name)
			continue
		}
		labels := r.GetLabels()
		if labels["policy.kyverno.io/synchronize"] == "enable" {
			if err := c.client.DeleteResource(r.GetAPIVersion(), r.GetKind(), r.GetNamespace(), r.GetName(), false); err != nil {
				logger.Error(err, "Generated resource is not deleted", "Resource", r.GetName())
			}
		}
	}
	logger.Info("deleting generate request", "name", gr.Name)
	// sync Handler will remove it from the queue
	c.enqueueGR(gr)
}

//Run ...
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	logger := c.log
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, c.pSynced, c.grSynced) {
		logger.Info("failed to sync informer cache")
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, constant.GenerateControllerResync, stopCh)
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
	logger := c.log
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		logger.Error(err, "failed to sync generate request", "key", key)
		c.queue.AddRateLimited(key)
		return
	}
	utilruntime.HandleError(err)
	logger.Error(err, "Dropping generate request from the queue", "key", key)
	c.queue.Forget(key)
}

func (c *Controller) syncGenerateRequest(key string) error {
	logger := c.log
	var err error
	startTime := time.Now()
	logger.Info("started sync", "key", key, "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished sync", "key", key, "processingTime", time.Since(startTime).String())
	}()
	_, grName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	gr, err := c.grLister.Get(grName)
	if err != nil {
		logger.Error(err, "failed to list generate requests")
		return err
	}
	return c.processGR(gr)
}
