package controller

import (
	"time"
	"fmt"

	"k8s.io/sample-controller/pkg/signals"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/tools/cache"

	clientset "nirmata/kube-policy/pkg/client/clientset/versioned"
	informers "nirmata/kube-policy/pkg/client/informers/externalversions"
	lister "nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
)

// Controller for CRD
type Controller struct {
	policyClientset clientset.Interface
	policyInformerFactory informers.SharedInformerFactory
	policyLister lister.PolicyLister
	policiesSynced cache.InformerSynced
	workqueue workqueue.RateLimitingInterface
}

// NewController from cmd args
func NewController(masterURL, kubeconfigPath string) (*Controller, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		return nil, err
	}

	policyClientset, err := clientset.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("Error building policy clientset: %v\n", err)
		return nil, err
	}

	policyInformerFactory := informers.NewSharedInformerFactory(policyClientset, time.Second*30)
	policyInformer := policyInformerFactory.Nirmata().V1alpha1().Policies()
	
	controller := &Controller {
		policyClientset: policyClientset,
		policyInformerFactory: policyInformerFactory,
		policyLister: policyInformer.Lister(),
		policiesSynced: policyInformer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Policies"),
	}

	policyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueue,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueue(new)
		},
		DeleteFunc: controller.enqueue,
	})

	return controller, nil
}

// Run is main controller thread
func (c *Controller) Run(threadiness int) error {
	stopCh := signals.SetupSignalHandler()
	c.policyInformerFactory.Start(stopCh)

	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	fmt.Println("Starting controller")

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
		time.Sleep(25 * time.Second)
		fmt.Println("I will wait here for 25 secs...")
	}
}

func (*Controller) enqueue(interface{}) {
    fmt.Println("I have found changes on Policy Resource")
}

// Idle : do nothing
func (*Controller)Idle() {
    fmt.Println("I'm controller, I do nothing")
}