package gencontroller

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/golang/glog"
	policyLister "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	policySharedInformer "github.com/nirmata/kyverno/pkg/sharedinformer"
	"github.com/nirmata/kyverno/pkg/violation"
	"k8s.io/apimachinery/pkg/api/errors"

	v1Informer "k8s.io/client-go/informers/core/v1"
	v1CoreLister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

//Controller watches the 'Namespace' resource creation/update and applied the generation rules on them
type Controller struct {
	client          *client.Client
	namespaceLister v1CoreLister.NamespaceLister
	namespaceSynced cache.InformerSynced
	policyLister    policyLister.PolicyLister
	workqueue       workqueue.RateLimitingInterface
}

//NewGenController returns a new Controller to manage generation rules
func NewGenController(client *client.Client,
	eventController event.Generator,
	policyInformer policySharedInformer.PolicyInformer,
	violationBuilder violation.Generator,
	namespaceInformer v1Informer.NamespaceInformer) *Controller {

	// create the controller
	controller := &Controller{
		client:          client,
		namespaceLister: namespaceInformer.Lister(),
		namespaceSynced: namespaceInformer.Informer().HasSynced,
		policyLister:    policyInformer.GetLister(),
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), wqNamespace),
	}
	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.createNamespaceHandler,
		UpdateFunc: controller.updateNamespaceHandler,
	})

	return controller
}
func (c *Controller) createNamespaceHandler(resource interface{}) {
	c.enqueueNamespace(resource)
}

func (c *Controller) updateNamespaceHandler(oldResoruce, newResource interface{}) {
	// DO we need to anything if the namespace is modified ?
}

func (c *Controller) enqueueNamespace(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		glog.Error(err)
		return
	}
	c.workqueue.Add(key)
}

//Run to run the controller
func (c *Controller) Run(stopCh <-chan struct{}) error {

	if ok := cache.WaitForCacheSync(stopCh, c.namespaceSynced); !ok {
		return fmt.Errorf("faield to wait for caches to sync")
	}

	for i := 0; i < workerCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	glog.Info("started namespace controller workers")
	return nil
}

//Stop to stop the controller
func (c *Controller) Stop() {
	defer c.workqueue.ShutDown()
	glog.Info("shutting down namespace controller workers")
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		err := c.syncHandler(obj)
		c.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		glog.Error(err)
		return true
	}
	return true
}

func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.workqueue.Forget(key)
		return
	}
	if c.workqueue.NumRequeues(key) < wqRetryLimit {
		glog.Warningf("Error syncing events %v: %v", key, err)
		c.workqueue.AddRateLimited(key)
		return
	}
	c.workqueue.Forget(key)
	glog.Error(err)
	glog.Warningf("Dropping the key %q out of the queue: %v", key, err)
}

func (c *Controller) syncHandler(obj interface{}) error {
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		return fmt.Errorf("expected string in workqueue but got %v", obj)
	}
	// Namespace is cluster wide resource
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.Errorf("invalid namespace key: %s", key)
		return err
	}
	// Get Namespace
	ns, err := c.namespaceLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("namespace '%s' in work queue no longer exists", key)
			return nil
		}
	}

	glog.Info("apply generation policy to resources :)")
	//TODO: need to find a way to store the policy such that we can directly queury the
	// policies with generation policies
	// PolicyListerExpansion
	c.processNamespace(ns)
	return nil
}
