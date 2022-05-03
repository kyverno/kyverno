package background

import (
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	common "github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	urkyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	urlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// Controller manages the life-cycle for Generate-Requests and applies generate rule
type Controller struct {
	// dynamic client implementation
	client dclient.Interface

	// typed client for Kyverno CRDs
	kyvernoClient kyvernoclient.Interface

	policyInformer kyvernoinformer.ClusterPolicyInformer

	// event generator interface
	eventGen event.Interface

	// urStatusControl is used to update UR status
	statusControl common.StatusControlInterface

	// UR that need to be synced
	queue workqueue.RateLimitingInterface

	// policyLister can list/get cluster policy from the shared informer's store
	policyLister kyvernolister.ClusterPolicyLister

	// policyLister can list/get Namespace policy from the shared informer's store
	npolicyLister kyvernolister.PolicyLister

	// urLister can list/get update request from the shared informer's store
	urLister urlister.UpdateRequestNamespaceLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister corelister.NamespaceLister

	log logr.Logger

	Config config.Interface
}

//NewController returns an instance of the Generate-Request Controller
func NewController(
	kubeClient kubernetes.Interface,
	kyvernoClient kyvernoclient.Interface,
	client dclient.Interface,
	policyInformer kyvernoinformer.ClusterPolicyInformer,
	npolicyInformer kyvernoinformer.PolicyInformer,
	urInformer urkyvernoinformer.UpdateRequestInformer,
	eventGen event.Interface,
	namespaceInformer coreinformers.NamespaceInformer,
	log logr.Logger,
	dynamicConfig config.Interface,
) (*Controller, error) {

	c := Controller{
		client:         client,
		kyvernoClient:  kyvernoClient,
		policyInformer: policyInformer,
		eventGen:       eventGen,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate-request"),
		log:            log,
		Config:         dynamicConfig,
	}

	c.statusControl = common.StatusControl{Client: kyvernoClient}
	urInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addUR,
		UpdateFunc: c.updateUR,
		DeleteFunc: c.deleteUR,
	})

	c.policyLister = policyInformer.Lister()
	c.npolicyLister = npolicyInformer.Lister()
	c.urLister = urInformer.Lister().UpdateRequests(config.KyvernoNamespace)
	c.nsLister = namespaceInformer.Lister()

	return &c, nil
}

// Run starts workers
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer c.log.Info("shutting down")

	c.policyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updatePolicy, // We only handle updates to policy
		// Deletion of policy will be handled by cleanup controller
	})

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
		c.queue.Forget(key)
		logger.V(4).Info("Dropping update request from the queue", "key", key, "error", err.Error())
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("retrying update request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to process update request", "key", key)
	c.queue.Forget(key)
}

func (c *Controller) syncUpdateRequest(key string) error {
	logger := c.log
	var err error
	startTime := time.Now()
	logger.V(4).Info("started sync", "key", key, "startTime", startTime)
	defer func() {
		logger.V(4).Info("completed sync update request", "key", key, "processingTime", time.Since(startTime).String())
	}()

	_, urName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	ur, err := c.urLister.Get(urName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		logger.Error(err, "failed to fetch update request", "key", key)
		return err
	}

	ok, err := c.MarkUR(ur)
	if !ok {
		logger.V(3).Info("another instance is handling the UR", "handler", ur.Status.Handler)
	}
	if err != nil {
		logger.Error(err, "failed to mark UR handler", "key", key)
		return err
	}

	if err := c.ProcessUR(ur); err != nil {
		logger.Error(err, "failed to process UR", "key", key)
		return err
	}

	return c.UnmarkUR(ur)
}

func (c *Controller) enqueueUpdateRequest(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "failed to extract name")
		return
	}

	c.log.V(5).Info("enqueued update request", "ur", key)
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

	var policyHasGenerate bool
	for _, rule := range autogen.ComputeRules(curP) {
		if rule.HasGenerate() {
			policyHasGenerate = true
		}
	}

	if reflect.DeepEqual(curP.Spec, oldP.Spec) {
		policyHasGenerate = false
	}

	if !policyHasGenerate {
		return
	}

	logger.V(4).Info("updating policy", "name", oldP.Name)

	urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(curP.Name)
	if err != nil {
		logger.Error(err, "failed to update request for policy", "name", curP.Name)
		return
	}

	// re-evaluate the UR as the policy was updated
	for _, ur := range urs {
		ur.Spec.Context.AdmissionRequestInfo.Operation = admissionv1.Update
		c.enqueueUpdateRequest(ur)
	}
}

func (c *Controller) addUR(obj interface{}) {
	ur := obj.(*urkyverno.UpdateRequest)
	c.enqueueUpdateRequest(ur)
}

func (c *Controller) updateUR(old, cur interface{}) {
	oldUr := old.(*urkyverno.UpdateRequest)
	curUr := cur.(*urkyverno.UpdateRequest)
	if oldUr.ResourceVersion == curUr.ResourceVersion {
		// Periodic resync will send update events for all known Namespace.
		// Two different versions of the same replica set will always have different RVs.
		return
	}
	// only process the ones that are in "Pending"/"Completed" state
	// if the UPDATE Request fails due to incorrect policy, it will be requeued during policy update
	if curUr.Status.State != urkyverno.Pending {
		return
	}
	c.enqueueUpdateRequest(curUr)
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
			logger.Info("tombstone contained object that is not a Update Request CR", "obj", obj)
			return
		}
	}

	if ur.Spec.GetRequestType() == urkyverno.Generate {
		for _, resource := range ur.Status.GeneratedResources {
			r, err := c.client.GetResource(resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "Generated resource is not deleted", "Resource", resource.Name)
				continue
			}

			if r != nil && r.GetLabels()["policy.kyverno.io/synchronize"] == "enable" {
				if err := c.client.DeleteResource(r.GetAPIVersion(), r.GetKind(), r.GetNamespace(), r.GetName(), false); err != nil && !apierrors.IsNotFound(err) {
					logger.Error(err, "Generated resource is not deleted", "Resource", r.GetName())
				}
			}
		}

		logger.V(3).Info("deleting update request", "name", ur.Name)
	}

	// sync Handler will remove it from the queue
	c.enqueueUpdateRequest(ur)
}
