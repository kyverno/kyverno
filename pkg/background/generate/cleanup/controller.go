package cleanup

import (
	"context"
	"strconv"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
)

type Controller interface {
	// Run starts workers
	Run(int, <-chan struct{})
}

// controller manages life-cycle of generate-requests
type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient kyvernoclient.Interface

	// informers
	pInformer  kyvernov1informers.ClusterPolicyInformer
	urInformer kyvernov1beta1informers.UpdateRequestInformer

	// listers
	pLister  kyvernov1listers.ClusterPolicyLister
	npLister kyvernov1listers.PolicyLister
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister
	nsLister corev1listers.NamespaceLister

	informersSynced []cache.InformerSynced

	// queue
	queue workqueue.RateLimitingInterface
}

// NewController returns a new controller instance to manage generate-requests
func NewController(
	kubeClient kubernetes.Interface,
	kyvernoclient kyvernoclient.Interface,
	client dclient.Interface,
	pInformer kyvernov1informers.ClusterPolicyInformer,
	npInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
	namespaceInformer corev1informers.NamespaceInformer,
) Controller {
	c := &controller{
		client:        client,
		kyvernoClient: kyvernoclient,
		pInformer:     pInformer,
		urInformer:    urInformer,
		pLister:       pInformer.Lister(),
		npLister:      npInformer.Lister(),
		urLister:      urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
		nsLister:      namespaceInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate-request-cleanup"),
	}

	c.informersSynced = []cache.InformerSynced{pInformer.Informer().HasSynced, npInformer.Informer().HasSynced, urInformer.Informer().HasSynced, namespaceInformer.Informer().HasSynced}
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("generate-request-cleanup", stopCh, c.informersSynced...) {
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

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.V(4).Info("deleting policy", "name", p.Name)

	generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(p, c.client, c.kyvernoClient, c.urLister, p.GetName(), logger)

	// get the generated resource name from update request for log
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		kyvernov1beta1.URGeneratePolicyLabel: p.Name,
	}))

	urList, err := c.urLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to get update request for the resource", "label", kyvernov1beta1.URGeneratePolicyLabel)
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

func (c *controller) deleteUR(obj interface{}) {
	ur, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1beta1.UpdateRequest)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	if ur.Status.Handler != "" {
		return
	}
	c.enqueue(ur)
}

func (c *controller) enqueue(ur *kyvernov1beta1.UpdateRequest) {
	key, err := cache.MetaNamespaceKeyFunc(ur)
	if err != nil {
		logger.Error(err, "failed to extract key")
		return
	}
	logger.V(5).Info("enqueue update request", "name", ur.Name)
	c.queue.Add(key)
}

// worker runs a worker thread that just de-queues items, processes them, and marks them done.
// It enforces that the syncUpdateRequest is never invoked concurrently with the same key.
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

func (c *controller) syncUpdateRequest(key string) error {
	logger := logger.WithValues("key", key)
	var err error
	startTime := time.Now()
	logger.V(4).Info("started syncing update request", "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncing update request", "processingTIme", time.Since(startTime).String())
	}()
	_, urName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	ur, err := c.urLister.Get(urName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("update request has been deleted")
			return nil
		}
		return err
	}
	if ur.Status.State == kyvernov1beta1.Pending {
		return nil
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
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
	}
	return c.processUR(*ur)
}

func (c *controller) processUR(ur kyvernov1beta1.UpdateRequest) error {
	logger := logger.WithValues("kind", ur.Kind, "namespace", ur.Namespace, "name", ur.Name)
	// 1- Corresponding policy has been deleted
	// then we don't delete the generated resources

	// 2- The trigger resource is deleted, then delete the generated resources
	if !ownerResourceExists(logger, c.client, ur) {
		deleteUR := false
		// check retry count in annotaion
		urAnnotations := ur.Annotations
		if val, ok := urAnnotations["generate.kyverno.io/retry-count"]; ok {
			retryCount, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				logger.Error(err, "unable to convert retry-count")
				return err
			}

			if retryCount >= 5 {
				deleteUR = true
			}
		}

		if deleteUR {
			if err := deleteGeneratedResources(logger, c.client, ur); err != nil {
				return err
			}
			// - trigger-resource is deleted
			// - generated-resources are deleted
			// - > Now delete the UpdateRequest CR
			return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
		}
	}
	return nil
}
