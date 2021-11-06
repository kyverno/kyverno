package generate

import (
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	"reflect"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/client-go/kubernetes"

	"github.com/go-logr/logr"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/resourcecache"
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

// Controller manages the life-cycle for Generate-Requests
type Controller struct {
	// dynamic client implementation
	client *dclient.Client

	// typed client for Kyverno CRDs
	kyvernoClient *kyvernoclient.Clientset

	policyInformer kyvernoinformer.ClusterPolicyInformer
	grInformer kyvernoinformer.GenerateRequestInformer

	// control is used to delete the GR
	control ControlInterface

	// event generator interface
	eventGen event.Interface

	// grStatusControl is used to update GR status
	statusControl StatusControlInterface

	// GR that need to be synced
	queue workqueue.RateLimitingInterface

	// policyLister can list/get cluster policy from the shared informer's store
	policyLister kyvernolister.ClusterPolicyLister

	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister

	// policySynced returns true if the Cluster policy store has been synced at least once
	policySynced cache.InformerSynced

	// grSynced returns true if the Generate Request store has been synced at least once
	grSynced cache.InformerSynced

	// dynamic shared informer factory
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory

	// only support Namespaces for re-evaluation on resource updates
	nsInformer informers.GenericInformer
	log        logr.Logger

	Config   config.Interface
	resCache resourcecache.ResourceCache
}

//NewController returns an instance of the Generate-Request Controller
func NewController(
	kubeClient kubernetes.Interface,
	kyvernoClient *kyvernoclient.Clientset,
	client *dclient.Client,
	policyInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	eventGen event.Interface,
	dynamicInformer dynamicinformer.DynamicSharedInformerFactory,
	log logr.Logger,
	dynamicConfig config.Interface,
	resourceCache resourcecache.ResourceCache,
) (*Controller, error) {

	c := Controller{
		client:          client,
		kyvernoClient:   kyvernoClient,
		policyInformer:  policyInformer,
		grInformer: grInformer,
		eventGen:        eventGen,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate-request"),
		dynamicInformer: dynamicInformer,
		log:             log,
		Config:          dynamicConfig,
		resCache:        resourceCache,
	}

	c.statusControl = StatusControl{client: kyvernoClient}
	c.control = Control{client: kyvernoClient}

	c.policySynced = policyInformer.Informer().HasSynced

	c.grSynced = grInformer.Informer().HasSynced
	//grInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc:    c.addGR,
	//	UpdateFunc: c.updateGR,
	//	DeleteFunc: c.deleteGR,
	//})

	grInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addGRNew,
		UpdateFunc: c.updateGRNew,
		DeleteFunc: c.deleteGRNew,
	})

	c.policyLister = policyInformer.Lister()
	c.grLister = grInformer.Lister().GenerateRequests(config.KyvernoNamespace)

	gvr, err := client.DiscoveryClient.GetGVRFromKind("Namespace")
	if err != nil {
		return nil, err
	}

	c.nsInformer = dynamicInformer.ForResource(gvr)

	return &c, nil
}

// Run starts workers
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer c.log.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, c.policySynced, c.grSynced) {
		c.log.Info("failed to sync informer cache")
		return
	}

	c.policyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updatePolicy, // We only handle updates to policy
		DeleteFunc: c.deletePolicy,
	})

	c.nsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updateGenericResource,
		DeleteFunc: c.deleteGenericResource,
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
		c.queue.Forget(key)
		logger.V(4).Info("Dropping generate request from the queue", "key", key, "error", err.Error())
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("retrying generate request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to process generate request", "key", key)
	c.queue.Forget(key)
}

func (c *Controller) syncGenerateRequest(key string) error {
	logger := c.log
	var err error
	startTime := time.Now()
	logger.V(4).Info("started sync", "key", key, "startTime", startTime)
	defer func() {
		logger.V(4).Info("completed sync generate request", "key", key, "processingTime", time.Since(startTime).String())
	}()

	_, grName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	gr, err := c.grLister.Get(grName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		logger.Error(err, "failed to fetch generate request", "key", key)
		return err
	}

	if gr.Spec.RequestType == "generate" {
		return c.processGR(gr)
	} else {
		_, err = c.policyLister.Get(gr.Spec.Policy)
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
		return c.processGCR(gr)
	}
}

func (c *Controller) updateGenericResource(old, cur interface{}) {
	logger := c.log
	curR := cur.(*unstructured.Unstructured)

	grs, err := c.grLister.GetGenerateRequestsForResource(curR.GetKind(), curR.GetNamespace(), curR.GetName())
	if err != nil {
		logger.Error(err, "failed to get generate request CR for the resource", "kind", curR.GetKind(), "name", curR.GetName(), "namespace", curR.GetNamespace())
		return
	}

	// re-evaluate the GR as the resource was updated
	for _, gr := range grs {
		gr.Spec.Context.AdmissionRequestInfo.Operation = v1beta1.Update
		c.enqueueGenerateRequest(gr)
	}
}

// EnqueueGenerateRequestFromWebhook - enqueueing generate requests from webhook
func (c *Controller) EnqueueGenerateRequestFromWebhook(gr *kyverno.GenerateRequest) {
	c.enqueueGenerateRequest(gr)
}

func (c *Controller) enqueueGenerateRequest(gr *kyverno.GenerateRequest) {
	c.log.V(5).Info("enqueuing generate request", "gr", gr.Name)
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

	var policyHasGenerate bool
	for _, rule := range curP.Spec.Rules {
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

	// get the list of GR for the current Policy version
	grs, err := c.grLister.GetGenerateRequestsForClusterPolicy(curP.Name)
	if err != nil {
		logger.Error(err, "failed to generate request for policy", "name", curP.Name)
		return
	}

	// re-evaluate the GR as the policy was updated
	for _, gr := range grs {
		gr.Spec.Context.AdmissionRequestInfo.Operation = v1beta1.Update
		c.enqueueGenerateRequest(gr)
	}
}

func (c *Controller) addGR(obj interface{}) {
	gr := obj.(*kyverno.GenerateRequest)
	gr.Spec.RequestType = "generate"
	c.enqueueGenerateRequest(gr)
}

func (c *Controller) addGCR(obj interface{}) {
	gr := obj.(*kyverno.GenerateRequest)
	gr.Spec.RequestType = "cleanup"
	c.enqueueGenerateRequest(gr)
}

func (c *Controller) addGRNew(obj interface{}) {
	c.addGR(obj)
	c.addGCR(obj)
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
	// if the Generate Request fails due to incorrect policy, it will be requeue during policy update
	if curGr.Status.State == kyverno.Failed {
		return
	}
	curGr.Spec.RequestType = "generate"
	c.enqueueGenerateRequest(curGr)
}

func (c *Controller) updateGCR(old, cur interface{}) {
	gr := cur.(*kyverno.GenerateRequest)
	gr.Spec.RequestType = "cleanup"
	c.enqueueGenerateRequest(gr)
}

func (c *Controller) updateGRNew(old, cur interface{}) {
	c.updateGR(old, cur)
	c.updateGCR(old, cur)
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

	logger.V(3).Info("deleting generate request", "name", gr.Name)

	// sync Handler will remove it from the queue
	gr.Spec.RequestType = "generate"
	c.enqueueGenerateRequest(gr)
}

func (c *Controller) deleteGCR(obj interface{}) {
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
	gr.Spec.RequestType = "cleanup"
	c.enqueueGenerateRequest(gr)
}

func (c * Controller) deleteGRNew(obj interface{}) {
	c.deleteGR(obj)
	c.deleteGCR(obj)
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
			c.addGCR(gr)
		}
	}
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
		gr.Spec.RequestType = "cleanup"
		c.enqueueGenerateRequest(gr)
	}
}
