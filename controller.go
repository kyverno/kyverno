package main

import (
	"time"
	"fmt"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/tools/cache"

	clientset "nirmata/kube-policy/pkg/client/clientset/versioned"
	informer "nirmata/kube-policy/pkg/client/informers/externalversions/policy/v1alpha1"
	lister "nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
)

// Controller for CRD
type Controller struct {
	policyClientset clientset.Interface
	policyLister lister.PolicyLister
	policiesSynced cache.InformerSynced
	workqueue workqueue.RateLimitingInterface
}

// NewController is used to create Controller
func NewController(clientset clientset.Interface, informer informer.PolicyInformer) *Controller {
    controller := &Controller {
		policyClientset: clientset,
		policyLister: informer.Lister(),
		policiesSynced: informer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Policies"),
	}

	// Set up an event handler for when Foo resources change
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueFoo,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueFoo(new)
		},
		DeleteFunc: controller.enqueueFoo,
	})

	return controller
}

// Run is main controller thread
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {

	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	fmt.Println("Starting Foo controller")

	if ok := cache.WaitForCacheSync(stopCh, c.policiesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	fmt.Println("Started workers")
	<-stopCh
	fmt.Println("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for {
		time.Sleep(5 * time.Second)
		fmt.Println("I will wait here for 5 secs...")
	}
}

func (*Controller) enqueueFoo(interface{}) {
    fmt.Println("I have found changes on Policy Resource")
}

// Idle : do nothing
func (*Controller)Idle() {
    fmt.Println("I'm controller, I do nothing")
}