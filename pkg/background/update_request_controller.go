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
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
)

type Controller interface {
	Run(int, <-chan struct{})
}

// controller manages the life-cycle for Generate-Requests and applies generate rule
type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient kyvernoclient.Interface

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

//NewController returns an instance of the Generate-Request Controller
func NewController(
	kubeClient kubernetes.Interface,
	kyvernoClient kyvernoclient.Interface,
	client dclient.Interface,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
	namespaceInformer corev1informers.NamespaceInformer,
	podInformer corev1informers.PodInformer,
	eventGen event.Interface,
	dynamicConfig config.Configuration,
) Controller {
	urLister := urInformer.Lister().UpdateRequests(config.KyvernoNamespace)
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

// Run starts workers
func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("background", stopCh, c.informersSynced...) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *controller) worker() {
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
		_, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return err
	}
	// if it was acquired by a pod that is gone, release it
	if ur.Status.Handler != "" {
		_, err = c.podLister.Pods(config.KyvernoNamespace).Get(ur.Status.Handler)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ur = ur.DeepCopy()
				ur.Status.Handler = ""
				_, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
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
			return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).DeleteCollection(
				context.TODO(),
				metav1.DeleteOptions{},
				metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)},
			)
		}
		return err
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
	key, err := cache.MetaNamespaceKeyFunc(kubeutils.GetObjectWithTombstone(obj))
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
	// sync Handler will remove it from the queue
	c.enqueueUpdateRequest(ur)
}

func (c *controller) processUR(ur *kyvernov1beta1.UpdateRequest) error {
	switch ur.Spec.Type {
	case kyvernov1beta1.Mutate:
		ctrl, _ := mutate.NewMutateExistingController(c.kyvernoClient, c.client, c.cpolLister, c.polLister, c.urLister, c.eventGen, logger, c.configuration)
		return ctrl.ProcessUR(ur)

	case kyvernov1beta1.Generate:
		ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client, c.cpolLister, c.polLister, c.urLister, c.eventGen, c.nsLister, logger, c.configuration)
		return ctrl.ProcessUR(ur)
	}
	return nil
}

func (c *controller) acquireUR(ur *kyvernov1beta1.UpdateRequest) (*kyvernov1beta1.UpdateRequest, bool, error) {
	name := ur.GetName()
	err := retry.RetryOnConflict(common.DefaultRetry, func() error {
		var err error
		ur, err = c.urLister.Get(name)
		if err != nil {
			return err
		}
		if ur.Status.Handler != "" {
			return nil
		}
		ur = ur.DeepCopy()
		ur.Status.Handler = config.KyvernoPodName
		ur, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		logger.Error(err, "failed to acquire ur", "name", name, "ur", ur)
		return nil, false, err
	}
	return ur, ur.Status.Handler == config.KyvernoPodName, err
}

func (c *controller) releaseUR(ur *kyvernov1beta1.UpdateRequest) (*kyvernov1beta1.UpdateRequest, error) {
	err := retry.RetryOnConflict(common.DefaultRetry, func() error {
		var err error
		ur, err = c.urLister.Get(ur.GetName())
		if err != nil {
			return err
		}
		if ur.Status.Handler != config.KyvernoPodName {
			return nil
		}
		ur = ur.DeepCopy()
		ur.Status.Handler = ""
		ur, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return err
	})
	return ur, err
}

func (c *controller) cleanUR(ur *kyvernov1beta1.UpdateRequest) error {
	if ur.Spec.Type == kyvernov1beta1.Mutate && ur.Status.State == kyvernov1beta1.Completed {
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
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
