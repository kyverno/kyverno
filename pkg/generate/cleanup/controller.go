package cleanup

import (
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/go-logr/logr"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
)

//Controller manages life-cycle of generate-requests
type Controller struct {

	// dynamic client implementation
	client *dclient.Client

	// typed client for kyverno CRDs
	kyvernoClient *kyvernoclient.Clientset

	pInformer  kyvernoinformer.ClusterPolicyInformer
	grInformer kyvernoinformer.GenerateRequestInformer

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

	// dynamic sharedinformer factory
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory

	// namespace informer
	nsInformer informers.GenericInformer

	// logger
	log logr.Logger
}

//NewController returns a new controller instance to manage generate-requests
func NewController(
	kubeClient kubernetes.Interface,
	kyvernoclient *kyvernoclient.Clientset,
	client *dclient.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory,
	log logr.Logger,
) (*Controller, error) {
	c := Controller{
		kyvernoClient:   kyvernoclient,
		client:          client,
		pInformer:       pInformer,
		grInformer:      grInformer,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate-request-cleanup"),
		dynamicInformer: dynamicInformer,
		log:             log,
	}

	c.control = Control{client: kyvernoclient}

	c.pLister = pInformer.Lister()
	c.grLister = grInformer.Lister().GenerateRequests(config.KyvernoNamespace)

	c.pSynced = pInformer.Informer().HasSynced
	c.grSynced = grInformer.Informer().HasSynced

	gvr, err := client.DiscoveryClient.GetGVRFromKind("Namespace")
	if err != nil {
		return nil, err
	}

	nsInformer := dynamicInformer.ForResource(gvr)
	c.nsInformer = nsInformer

	return &c, nil
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
		c.enqueue(gr)
	}
}

func (c *Controller) deletePolicy(obj interface{}) {
	logger := c.log
	p, ok := obj.(*kyverno.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("couldn't get object from tombstone", "obj", obj)
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
	rules := p.Spec.Rules

	generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(rules, c.client, p.GetName(), logger)

	// get the generated resource name from generate request for log
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		"generate.kyverno.io/policy-name": p.Name,
	}))

	grList, err := c.grLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to get generate request for the resource", "label", "generate.kyverno.io/policy-name")
		return
	}

	for _, gr := range grList {
		for _, generatedResource := range gr.Status.GeneratedResources {
			logger.V(4).Info("retaining resource", "apiVersion", generatedResource.APIVersion, "kind", generatedResource.Kind, "name", generatedResource.Name, "namespace", generatedResource.Namespace)
		}
	}

	if !generatePolicyWithClone {
		grs, err := c.grLister.GetGenerateRequestsForClusterPolicy(p.Name)
		if err != nil {
			logger.Error(err, "failed to generate request for the policy", "name", p.Name)
			return
		}

		for _, gr := range grs {
			logger.V(4).Info("enqueue the gr for cleanup", "gr name", gr.Name)
			c.addGR(gr)
		}
	}
}

func (c *Controller) addGR(obj interface{}) {
	gr := obj.(*kyverno.GenerateRequest)
	c.enqueue(gr)
}

func (c *Controller) updateGR(old, cur interface{}) {
	gr := cur.(*kyverno.GenerateRequest)
	c.enqueue(gr)
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
		r, err := c.client.GetResource(resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to fetch generated resource", "resource", resource.Name)
			return
		}

		if r != nil && r.GetLabels()["policy.kyverno.io/synchronize"] == "enable" {
			if err := c.client.DeleteResource(r.GetAPIVersion(), r.GetKind(), r.GetNamespace(), r.GetName(), false); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to delete the generated resource", "resource", r.GetName())
				return
			}
		}
	}

	logger.V(4).Info("deleting Generate Request CR", "name", gr.Name)
	// sync Handler will remove it from the queue
	c.enqueue(gr)
}

func (c *Controller) enqueue(gr *kyverno.GenerateRequest) {
	// skip enqueueing Pending requests
	if gr.Status.State == kyverno.Pending {
		return
	}

	logger := c.log
	key, err := cache.MetaNamespaceKeyFunc(gr)
	if err != nil {
		logger.Error(err, "failed to extract key")
		return
	}

	logger.V(5).Info("enqueue generate request", "name", gr.Name)
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

	c.pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deletePolicy, // we only cleanup if the policy is delete
	})

	c.grInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addGR,
		UpdateFunc: c.updateGR,
		DeleteFunc: c.deleteGR,
	})

	c.nsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deleteGenericResource,
	})

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just de-queues items, processes them, and marks them done.
// It enforces that the syncGenerateRequest is never invoked concurrently with the same key.
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
	err := c.syncGenerateRequest(key.(string))
	c.handleErr(err, key)

	return true
}

func (c *Controller) handleErr(err error, key interface{}) {
	logger := c.log
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if apierrors.IsNotFound(err) {
		logger.V(4).Info("dropping generate request", "key", key, "error", err.Error())
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("retrying generate request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to cleanup generate request", "key", key)
	c.queue.Forget(key)
}

func (c *Controller) syncGenerateRequest(key string) error {
	logger := c.log.WithValues("key", key)
	var err error
	startTime := time.Now()
	logger.V(4).Info("started syncing generate request", "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncing generate request", "processingTIme", time.Since(startTime).String())
	}()
	_, grName, err := cache.SplitMetaNamespaceKey(key)
	if apierrors.IsNotFound(err) {
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
		if !apierrors.IsNotFound(err) {
			return err
		}
		err = c.control.Delete(gr.Name)
		if err != nil {
			return err
		}
		return nil
	}
	return c.processGR(*gr)
}
