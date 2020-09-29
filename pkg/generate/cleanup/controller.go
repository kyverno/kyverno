package cleanup

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

//Controller manages life-cycle of generate-requests
type Controller struct {
	ctx context.Context
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
	log        logr.Logger
}

//NewController returns a new controller instance to manage generate-requests
func NewController(
	ctx context.Context,
	kyvernoclient *kyvernoclient.Clientset,
	client *dclient.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory,
	log logr.Logger,
) *Controller {
	c := Controller{
		ctx:           ctx,
		kyvernoClient: kyvernoclient,
		client:        client,
		//TODO: do the math for worst case back off and make sure cleanup runs after that
		// as we dont want a deleted GR to be re-queue
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(1, 30), "generate-request-cleanup"),
		dynamicInformer: dynamicInformer,
		log:             log,
	}
	c.control = Control{client: kyvernoclient}
	c.enqueueGR = c.enqueue
	c.syncHandler = c.syncGenerateRequest

	c.pLister = pInformer.Lister()
	c.grLister = grInformer.Lister().GenerateRequests(config.KubePolicyNamespace)

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
	logger := c.log
	r := obj.(*unstructured.Unstructured)
	grs, err := c.grLister.GetGenerateRequestsForResource(r.GetKind(), r.GetNamespace(), r.GetName())
	if err != nil {
		logger.Error(err, "failed to get generate request CR for resource", "kind", r.GetKind(), "namespace", r.GetNamespace(), "name", r.GetName())
		return
	}
	// re-evaluate the GR as the resource was deleted
	for _, gr := range grs {
		c.enqueueGR(gr)
	}
}

func (c *Controller) deletePolicy(obj interface{}) {
	logger := c.log
	p, ok := obj.(*kyverno.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("ouldn't get object from tombstone", "obj", obj)
			return
		}
		_, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
		if !ok {
			logger.Info("Tombstone contained object that is not a Generate Request", "obj", obj)
			return
		}
	}
	logger.V(4).Info("deleting policy", "name", p.Name)
	// clean up the GR
	// Get the corresponding GR
	// get the list of GR for the current Policy version
	grs, err := c.grLister.GetGenerateRequestsForClusterPolicy(p.Name)
	if err != nil {
		logger.Error(err, "failed to generate request CR for the policy", "name", p.Name)
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
			logger.Info("ombstone contained object that is not a Generate Request", "obj", obj)
			return
		}
	}
	for _, resource := range gr.Status.GeneratedResources {
		r, err := c.client.GetResource(c.ctx, resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
		if err != nil {
			logger.Error(err, "Generated resource is not deleted", "Resource", resource.Name)
			return
		}
		labels := r.GetLabels()
		if labels["policy.kyverno.io/synchronize"] == "enable" {
			if err := c.client.DeleteResource(c.ctx, r.GetAPIVersion(), r.GetKind(), r.GetNamespace(), r.GetName(), false); err != nil {
				logger.Error(err, "Generated resource is not deleted", "Resource", r.GetName())
				return
			}
		}
	}
	logger.V(4).Info("deleting Generate Request CR", "name", gr.Name)
	// sync Handler will remove it from the queue
	c.enqueueGR(gr)
}

func (c *Controller) enqueue(gr *kyverno.GenerateRequest) {
	logger := c.log
	key, err := cache.MetaNamespaceKeyFunc(gr)
	if err != nil {
		logger.Error(err, "failed to extract key")
		return
	}
	logger.V(4).Info("eneque generate request", "name", gr.Name)
	c.queue.Add(key)
}

//Run starts the generate-request re-conciliation loop
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
		go wait.Until(c.worker, constant.GenerateRequestControllerResync, stopCh)
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
	logger.Error(err, "dropping generate request out of the queue", "key", key)
	c.queue.Forget(key)
}

func (c *Controller) syncGenerateRequest(key string) error {
	logger := c.log.WithValues("key", key)
	var err error
	startTime := time.Now()
	logger.Info("started syncing generate request", "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncying generate request", "processingTIme", time.Since(startTime).String())
	}()
	_, grName, err := cache.SplitMetaNamespaceKey(key)
	if errors.IsNotFound(err) {
		logger.Info("generate request has been deleted")
		return nil
	}
	if err != nil {
		return err
	}
	gr, err := c.grLister.Get(grName)
	if err != nil {
		return err
	}

	_, err = c.pLister.Get(gr.Spec.Policy)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		c.control.Delete(gr.Name)
		return nil
	}
	return c.processGR(*gr)
}
