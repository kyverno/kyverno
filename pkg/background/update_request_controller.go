package background

import (
	"context"
	"fmt"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/background/mutate"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
)

type Controller interface {
	// Run starts workers
	Run(context.Context, int)
}

// controller manages the life-cycle for Generate-Requests and applies generate rule
type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister
	urLister   kyvernov1beta1listers.UpdateRequestNamespaceLister
	nsLister   corev1listers.NamespaceLister
	podLister  corev1listers.PodLister

	informersSynced []cache.InformerSynced

	// queue
	queue workqueue.RateLimitingInterface

	eventGen      event.Interface
	configuration config.Configuration
}

// NewController returns an instance of the Generate-Request Controller
func NewController(
	kyvernoClient versioned.Interface,
	client dclient.Interface,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
	namespaceInformer corev1informers.NamespaceInformer,
	podInformer corev1informers.PodInformer,
	eventGen event.Interface,
	dynamicConfig config.Configuration,
) Controller {
	urLister := urInformer.Lister().UpdateRequests(config.KyvernoNamespace())
	c := controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		cpolLister:    cpolInformer.Lister(),
		polLister:     polInformer.Lister(),
		urLister:      urLister,
		nsLister:      namespaceInformer.Lister(),
		podLister:     podInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "update-request"),
		eventGen:      eventGen,
		configuration: dynamicConfig,
	}
	urInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addUR,
		UpdateFunc: c.updateUR,
		DeleteFunc: c.deleteUR,
	})
	cpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updatePolicy,
		DeleteFunc: c.deletePolicy,
	})
	polInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updatePolicy,
		DeleteFunc: c.deletePolicy,
	})

	c.informersSynced = []cache.InformerSynced{cpolInformer.Informer().HasSynced, polInformer.Informer().HasSynced, urInformer.Informer().HasSynced, namespaceInformer.Informer().HasSynced, podInformer.Informer().HasSynced}

	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("background", ctx.Done(), c.informersSynced...) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}

	<-ctx.Done()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *controller) worker(ctx context.Context) {
	for c.processNextWorkItem() {
	}
}

func (c *controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)
	err := c.syncUpdateRequest(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *controller) handleErr(err error, key interface{}) {
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

func (c *controller) syncUpdateRequest(key string) error {
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
		return err
	}

	// if not in any state, try to set it to pending
	if ur.Status.State == "" {
		ur = ur.DeepCopy()
		ur.Status.State = kyvernov1beta1.Pending
		_, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return err
	}
	// if it was acquired by a pod that is gone, release it
	if ur.Status.Handler != "" {
		_, err = c.podLister.Pods(config.KyvernoNamespace()).Get(ur.Status.Handler)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ur = ur.DeepCopy()
				ur.Status.Handler = ""
				_, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
			}
			return err
		}
	}
	// try to get the linked policy
	if _, err := c.getPolicy(ur.Spec.Policy); err != nil {
		if apierrors.IsNotFound(err) && ur.Spec.Type == kyvernov1beta1.Mutate {
			// here only takes care of mutateExisting policies
			// generate cleanup controller handles policy deletion
			selector := &metav1.LabelSelector{
				MatchLabels: common.MutateLabelsSet(ur.Spec.Policy, nil),
			}
			return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).DeleteCollection(
				context.TODO(),
				metav1.DeleteOptions{},
				metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)},
			)
		}
		// check if cleanup is required in case policy is no longer exists
		if err := c.checkIfCleanupRequired(ur); err != nil {
			return err
		}
	}
	// if in pending state, try to acquire ur and eventually process it
	if ur.Status.State == kyvernov1beta1.Pending {
		ur, ok, err := c.acquireUR(ur)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to mark handler for UR %s: %v", key, err)
		}
		if !ok {
			logger.V(3).Info("another instance is handling the UR", "handler", ur.Status.Handler)
			return nil
		}
		logger.V(3).Info("UR is marked successfully", "ur", ur.GetName(), "resourceVersion", ur.GetResourceVersion())
		if err := c.processUR(ur); err != nil {
			return fmt.Errorf("failed to process UR %s: %v", key, err)
		}
	}
	ur, err = c.releaseUR(ur)
	if err != nil {
		return fmt.Errorf("failed to unmark UR %s: %v", key, err)
	}
	err = c.cleanUR(ur)
	return err
}

func (c *controller) checkIfCleanupRequired(ur *kyvernov1beta1.UpdateRequest) error {
	var err error
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(ur.Spec.Policy)
	if err != nil {
		return err
	}

	if pNamespace == "" {
		_, err = c.cpolLister.Get(pName)
	} else {
		_, err = c.polLister.Policies(pNamespace).Get(pName)
	}
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		logger.V(4).Info("policy no longer exists, deleting the update request and respective resource based on synchronize", "ur", ur.Name, "policy", ur.Spec.Policy)
		for _, e := range ur.Status.GeneratedResources {
			if err := c.cleanupDataResource(e); err != nil {
				logger.Error(err, "failed to clean up data resource on policy deletion")
			}
		}
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
	}
	return nil
}

// cleanupDataResource deletes resource if sync is enabled for data policy
func (c *controller) cleanupDataResource(targetSpec kyvernov1.ResourceSpec) error {
	target, err := c.client.GetResource(targetSpec.APIVersion, targetSpec.Kind, targetSpec.Namespace, targetSpec.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to find generated resource %s/%s: %v", targetSpec.Namespace, targetSpec.Name, err)
		}
	}

	if target == nil {
		return nil
	}

	labels := target.GetLabels()
	syncEnabled := labels["policy.kyverno.io/synchronize"] == "enable"
	clone := labels["generate.kyverno.io/clone-policy-name"] != ""

	if syncEnabled && !clone {
		if err := c.client.DeleteResource(target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName(), false); err != nil {
			return fmt.Errorf("failed to delete data resource %s/%s: %v", targetSpec.Namespace, targetSpec.Name, err)
		}
	}
	return nil
}

func (c *controller) enqueueUpdateRequest(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract name")
		return
	}
	logger.V(5).Info("enqueued update request", "ur", key)
	c.queue.Add(key)
}

func (c *controller) updatePolicy(_, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to compute policy key")
	} else {
		logger.V(4).Info("updating policy", "key", key)
		urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(key)
		if err != nil {
			logger.Error(err, "failed to list update requests for policy", "key", key)
			return
		}
		// re-evaluate the UR as the policy was updated
		for _, ur := range urs {
			c.enqueueUpdateRequest(ur)
		}
	}
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.V(4).Info("deleting policy", "name", p.Name)
	key, err := cache.MetaNamespaceKeyFunc(kubeutils.GetObjectWithTombstone(obj))
	if err != nil {
		logger.Error(err, "failed to compute policy key")
	} else {
		logger.V(4).Info("updating policy", "key", key)

		// check if deleted policy is clone generate policy
		generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(p, c.client, c.kyvernoClient, c.urLister, p.GetName(), logger)

		// get the generated resource name from update request
		selector := labels.SelectorFromSet(labels.Set(map[string]string{
			kyvernov1beta1.URGeneratePolicyLabel: p.Name,
		}))

		urList, err := c.urLister.List(selector)
		if err != nil {
			logger.Error(err, "failed to get update request for the resource", "label", kyvernov1beta1.URGeneratePolicyLabel)
			return
		}

		if !generatePolicyWithClone {
			// re-evaluate the UR as the policy was updated
			for _, ur := range urList {
				logger.V(4).Info("enqueue the ur for cleanup", "ur name", ur.Name)
				c.enqueueUpdateRequest(ur)
			}
		} else {
			for _, ur := range urList {
				for _, generatedResource := range ur.Status.GeneratedResources {
					logger.V(4).Info("retaining resource for cloned policy", "apiVersion", generatedResource.APIVersion, "kind", generatedResource.Kind, "name", generatedResource.Name, "namespace", generatedResource.Namespace)
				}
			}
		}
	}
}

func (c *controller) addUR(obj interface{}) {
	ur := obj.(*kyvernov1beta1.UpdateRequest)
	c.enqueueUpdateRequest(ur)
}

func (c *controller) updateUR(_, cur interface{}) {
	curUr := cur.(*kyvernov1beta1.UpdateRequest)
	c.enqueueUpdateRequest(curUr)
}

func (c *controller) deleteUR(obj interface{}) {
	ur, ok := obj.(*kyvernov1beta1.UpdateRequest)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("Couldn't get object from tombstone", "obj", obj)
			return
		}
		ur, ok = tombstone.Obj.(*kyvernov1beta1.UpdateRequest)
		if !ok {
			logger.Info("tombstone contained object that is not a Update Request CR", "obj", obj)
			return
		}
	}
	if ur.Status.Handler != "" {
		return
	}
	// sync Handler will remove it from the queue
	c.enqueueUpdateRequest(ur)
}

func (c *controller) processUR(ur *kyvernov1beta1.UpdateRequest) error {
	statusControl := common.NewStatusControl(c.kyvernoClient, c.urLister)
	switch ur.Spec.Type {
	case kyvernov1beta1.Mutate:
		ctrl := mutate.NewMutateExistingController(c.client, statusControl, c.cpolLister, c.polLister, c.configuration, c.eventGen, logger)
		return ctrl.ProcessUR(ur)
	case kyvernov1beta1.Generate:
		ctrl := generate.NewGenerateController(c.client, c.kyvernoClient, statusControl, c.cpolLister, c.polLister, c.urLister, c.nsLister, c.configuration, c.eventGen, logger)
		return ctrl.ProcessUR(ur)
	}
	return nil
}

func (c *controller) acquireUR(ur *kyvernov1beta1.UpdateRequest) (*kyvernov1beta1.UpdateRequest, bool, error) {
	name := ur.GetName()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		ur, err = c.urLister.Get(name)
		if err != nil {
			return err
		}
		if ur.Status.Handler != "" {
			return nil
		}
		ur = ur.DeepCopy()
		ur.Status.Handler = config.KyvernoPodName()
		ur, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		logger.Error(err, "failed to acquire ur", "name", name, "ur", ur)
		return nil, false, err
	}
	return ur, ur.Status.Handler == config.KyvernoPodName(), err
}

func (c *controller) releaseUR(ur *kyvernov1beta1.UpdateRequest) (*kyvernov1beta1.UpdateRequest, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		ur, err = c.urLister.Get(ur.GetName())
		if err != nil {
			return err
		}
		if ur.Status.Handler != config.KyvernoPodName() {
			return nil
		}
		ur = ur.DeepCopy()
		ur.Status.Handler = ""
		ur, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return err
	})
	return ur, err
}

func (c *controller) cleanUR(ur *kyvernov1beta1.UpdateRequest) error {
	if ur.Spec.Type == kyvernov1beta1.Mutate && ur.Status.State == kyvernov1beta1.Completed {
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
	}
	return nil
}

func (c *controller) getPolicy(key string) (kyvernov1.PolicyInterface, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		return c.cpolLister.Get(name)
	}
	return c.polLister.Policies(namespace).Get(name)
}
