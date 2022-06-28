package cleanup

import (
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	urkyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	urkyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
)

//Controller manages life-cycle of generate-requests
type Controller struct {

	// dynamic client implementation
	client dclient.Interface

	// typed client for kyverno CRDs
	kyvernoClient kyvernoclient.Interface

	pInformer  kyvernoinformer.ClusterPolicyInformer
	urInformer urkyvernoinformer.UpdateRequestInformer

	// control is used to delete the UR
	control ControlInterface

	// ur that need to be synced
	queue workqueue.RateLimitingInterface

	// pLister can list/get cluster policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernolister.PolicyLister

	// urLister can list/get update request from the shared informer's store
	urLister urkyvernolister.UpdateRequestNamespaceLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister corelister.NamespaceLister

	informersSynced []cache.InformerSynced

	// logger
	log logr.Logger
}

//NewController returns a new controller instance to manage generate-requests
func NewController(
	kubeClient kubernetes.Interface,
	kyvernoclient kyvernoclient.Interface,
	client dclient.Interface,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	npInformer kyvernoinformer.PolicyInformer,
	urInformer urkyvernoinformer.UpdateRequestInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	log logr.Logger,
) (*Controller, error) {
	c := Controller{
		kyvernoClient: kyvernoclient,
		client:        client,
		pInformer:     pInformer,
		urInformer:    urInformer,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate-request-cleanup"),
		log:           log,
	}

	c.control = Control{client: kyvernoclient}

	c.pLister = pInformer.Lister()
	c.npLister = npInformer.Lister()
	c.urLister = urInformer.Lister().UpdateRequests(config.KyvernoNamespace)
	c.nsLister = namespaceInformer.Lister()

	c.informersSynced = []cache.InformerSynced{pInformer.Informer().HasSynced, npInformer.Informer().HasSynced, urInformer.Informer().HasSynced, namespaceInformer.Informer().HasSynced}

	return &c, nil
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
		p, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
		if !ok {
			logger.Info("Tombstone contained object that is not a Update Request", "obj", obj)
			return
		}
	}

	logger.V(4).Info("deleting policy", "name", p.Name)

	generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(p, c.client, c.kyvernoClient, c.urLister, p.GetName(), logger)

	// get the generated resource name from update request for log
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		urkyverno.URGeneratePolicyLabel: p.Name,
	}))

	urList, err := c.urLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to get update request for the resource", "label", urkyverno.URGeneratePolicyLabel)
		return
	}

	for _, ur := range urList {
		for _, generatedResource := range ur.Status.GeneratedResources {
			logger.V(4).Info("retaining resource", "apiVersion", generatedResource.APIVersion, "kind", generatedResource.Kind, "name", generatedResource.Name, "namespace", generatedResource.Namespace)
		}
	}

	if !generatePolicyWithClone {
		urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(p.Name)
		if err != nil {
			logger.Error(err, "failed to update request for the policy", "name", p.Name)
			return
		}

		for _, ur := range urs {
			logger.V(4).Info("enqueue the ur for cleanup", "ur name", ur.Name)
			c.enqueue(ur)
		}
	}
}

func (c *Controller) deleteUR(obj interface{}) {
	logger := c.log
	ur, ok := obj.(*urkyverno.UpdateRequest)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("Couldn't get object from tombstone", "obj", obj)
			return
		}
		ur, ok = tombstone.Obj.(*urkyverno.UpdateRequest)
		if !ok {
			logger.Info("ombstone contained object that is not a Update Request", "obj", obj)
			return
		}
	}

	if ur.Status.Handler != "" {
		return
	}

	c.enqueue(ur)
}

func (c *Controller) enqueue(ur *urkyverno.UpdateRequest) {
	// skip enqueueing Pending requests
	if ur.Status.State == urkyverno.Pending {
		return
	}

	logger := c.log
	key, err := cache.MetaNamespaceKeyFunc(ur)
	if err != nil {
		logger.Error(err, "failed to extract key")
		return
	}

	logger.V(5).Info("enqueue update request", "name", ur.Name)
	c.queue.Add(key)
}

//Run starts the update-request re-conciliation loop
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	logger := c.log
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("update-request-cleanup", stopCh, c.informersSynced...) {
		return
	}

	c.pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deletePolicy, // we only cleanup if the policy is delete
	})

	c.urInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deleteUR,
	})

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just de-queues items, processes them, and marks them done.
// It enforces that the syncUpdateRequest is never invoked concurrently with the same key.
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
	err := c.syncUpdateRequest(key.(string))
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
		logger.V(4).Info("dropping update request", "key", key, "error", err.Error())
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("retrying update request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to cleanup update request", "key", key)
	c.queue.Forget(key)
}

func (c *Controller) syncUpdateRequest(key string) error {
	logger := c.log.WithValues("key", key)
	var err error
	startTime := time.Now()
	logger.V(4).Info("started syncing update request", "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncing update request", "processingTIme", time.Since(startTime).String())
	}()
	_, urName, err := cache.SplitMetaNamespaceKey(key)
	if apierrors.IsNotFound(err) {
		logger.Info("update request has been deleted")
		return nil
	}
	if err != nil {
		return err
	}
	ur, err := c.urLister.Get(urName)
	if err != nil {
		return err
	}

	pNamespace, pName, err := cache.SplitMetaNamespaceKey(ur.Spec.Policy)
	if err != nil {
		return err
	}

	if pNamespace == "" {
		_, err = c.pLister.Get(pName)

	} else {
		_, err = c.npLister.Policies(pNamespace).Get(pName)
	}

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		logger.Error(err, "failed to get policy, deleting the update request", "key", ur.Spec.Policy)
		return c.control.Delete(ur.Name)
	}

	return c.processUR(*ur)
}
