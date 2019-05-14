package controller

import (
	"fmt"
	"log"
	"os"
	"time"

	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	policyclientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	infomertypes "github.com/nirmata/kube-policy/pkg/client/informers/externalversions/policy/v1alpha1"
	lister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	event "github.com/nirmata/kube-policy/pkg/event"
	violation "github.com/nirmata/kube-policy/pkg/violation"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

//PolicyController to manage Policy CRD
type PolicyController struct {
	kubeClient       *kubeClient.KubeClient
	policyLister     lister.PolicyLister
	policyInterface  policyclientset.Interface
	policySynced     cache.InformerSynced
	violationBuilder violation.Generator
	eventBuilder     event.Generator
	logger           *log.Logger
	queue            workqueue.RateLimitingInterface
}

// NewPolicyController from cmd args
func NewPolicyController(policyInterface policyclientset.Interface,
	policyInformer infomertypes.PolicyInformer,
	violationBuilder violation.Generator,
	eventController event.Generator,
	logger *log.Logger,
	kubeClient *kubeClient.KubeClient) *PolicyController {

	if logger == nil {
		logger = log.New(os.Stdout, "Policy Controller: ", log.LstdFlags)
	}

	controller := &PolicyController{
		kubeClient:       kubeClient,
		policyLister:     policyInformer.Lister(),
		policyInterface:  policyInterface,
		policySynced:     policyInformer.Informer().HasSynced,
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

func (pc *PolicyController) createPolicyHandler(resource interface{}) {
	pc.enqueuePolicy(resource)
}

func (pc *PolicyController) updatePolicyHandler(oldResource, newResource interface{}) {
	newPolicy := newResource.(*types.Policy)
	oldPolicy := oldResource.(*types.Policy)
	if newPolicy.ResourceVersion == oldPolicy.ResourceVersion {
		return
	}
	pc.enqueuePolicy(newResource)
}

func (pc *PolicyController) deletePolicyHandler(resource interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = resource.(metav1.Object); !ok {
		utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
		return
	}
	pc.logger.Printf("policy deleted: %s", object.GetName())
}

func (pc *PolicyController) enqueuePolicy(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	pc.queue.Add(key)
}

// Run is main controller thread
func (pc *PolicyController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, pc.policySynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < policyControllerWorkerCount; i++ {
		go wait.Until(pc.runWorker, time.Second, stopCh)
	}

	pc.logger.Println("Started policy controller")
	return nil
}

func (pc *PolicyController) runWorker() {
	for pc.processNextWorkItem() {
	}
}

func (pc *PolicyController) processNextWorkItem() bool {
	obj, shutdown := pc.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer pc.queue.Done(obj)
		err := pc.syncHandler(obj)
		pc.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

func (pc *PolicyController) handleErr(err error, key interface{}) {
	if err == nil {
		pc.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if pc.queue.NumRequeues(key) < policyWorkQueueRetryLimit {
		pc.logger.Printf("Error syncing events %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		pc.queue.AddRateLimited(key)
		return
	}
	pc.queue.Forget(key)
	utilruntime.HandleError(err)
	pc.logger.Printf("Dropping the key %q out of the queue: %v", key, err)
}

func (pc *PolicyController) syncHandler(obj interface{}) error {
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
	policy, err := pc.policyLister.Policies(namespace).Get(name)
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
