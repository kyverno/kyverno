package controller

import (
	"fmt"
	"log"
	"time"

	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	policyclientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	infomertypes "github.com/nirmata/kube-policy/pkg/client/informers/externalversions/policy/v1alpha1"
	lister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	engine "github.com/nirmata/kube-policy/pkg/engine"
	event "github.com/nirmata/kube-policy/pkg/event"
	violation "github.com/nirmata/kube-policy/pkg/violation"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

//Controller to manage Policy CRD
type Controller struct {
	kubeClient       *kubeClient.KubeClient
	policyLister     lister.PolicyLister
	policyInterface  policyclientset.Interface
	policySynced     cache.InformerSynced
	policyEngine     engine.PolicyEngine
	violationBuilder violation.Generator
	eventBuilder     event.Generator
	logger           *log.Logger
	queue            workqueue.RateLimitingInterface
}

// NewPolicyController from cmd args
func NewPolicyController(policyInterface policyclientset.Interface,
	policyInformer infomertypes.PolicyInformer,
	policyEngine engine.PolicyEngine,
	violationBuilder violation.Generator,
	eventController event.Generator,
	logger *log.Logger,
	kubeClient *kubeClient.KubeClient) *Controller {

	controller := &Controller{
		kubeClient:       kubeClient,
		policyLister:     policyInformer.Lister(),
		policyInterface:  policyInterface,
		policySynced:     policyInformer.Informer().HasSynced,
		policyEngine:     policyEngine,
		violationBuilder: violationBuilder,
		eventBuilder:     eventController,
		logger:           logger,
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), policyWorkQueueName),
	}

	policyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.createPolicyHandler,
		UpdateFunc: controller.updatePolicyHandler,
		DeleteFunc: controller.deletePolicyHandler,
	})
	return controller
}

func (c *Controller) createPolicyHandler(resource interface{}) {
	c.enqueuePolicy(resource)
}

func (c *Controller) updatePolicyHandler(oldResource, newResource interface{}) {
	newPolicy := newResource.(*types.Policy)
	oldPolicy := oldResource.(*types.Policy)
	if newPolicy.ResourceVersion == oldPolicy.ResourceVersion {
		return
	}
	c.enqueuePolicy(newResource)
}

func (c *Controller) deletePolicyHandler(resource interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = resource.(metav1.Object); !ok {
		utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
		return
	}
	c.logger.Printf("policy deleted: %s", object.GetName())
}

func (c *Controller) enqueuePolicy(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// Run is main controller thread
func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	c.logger.Printf("starting policy controller")

	c.logger.Printf("waiting for infomer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.policySynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.logger.Println("starting policy controller workers")
	for i := 0; i < policyControllerWorkerCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	c.logger.Println("started policy controller workers")
	<-stopCh
	c.logger.Println("shutting down policy controller workers")
	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.queue.Done(obj)
		err := c.syncHandler(obj)
		c.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < policyWorkQueueRetryLimit {
		c.logger.Printf("Error syncing events %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	utilruntime.HandleError(err)
	c.logger.Printf("Dropping the key %q out of the queue: %v", key, err)
}

func (c *Controller) syncHandler(obj interface{}) error {
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		return fmt.Errorf("expected string in workqueue but got %#v", obj)
	}
	// convert the namespace/name string into distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid policy key: %s", key))
		return nil
	}

	// Get Policy resource with namespace/name
	policy, err := c.policyLister.Policies(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("policy '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}
	// process policy on existing resource
	// get the violations and pass to violation Builder
	// get the events and pass to event Builder
	fmt.Println(policy)
	return nil
}
