package event

import (
	"errors"
	"fmt"
	"log"
	"time"

	controllerinternalinterfaces "github.com/nirmata/kube-policy/controller/internalinterfaces"
	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/pkg/client/clientset/versioned/scheme"
	policyscheme "github.com/nirmata/kube-policy/pkg/client/clientset/versioned/scheme"
	"github.com/nirmata/kube-policy/pkg/event/internalinterfaces"
	utils "github.com/nirmata/kube-policy/pkg/event/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type builder struct {
	kubeClient   *kubeClient.KubeClient
	controller   controllerinternalinterfaces.PolicyGetter
	workqueue    workqueue.RateLimitingInterface
	recorder     record.EventRecorder
	logger       *log.Logger
	policySynced cache.InformerSynced
}

type Builder interface {
	internalinterfaces.BuilderInternal
	SyncHandler(key utils.EventInfo) error
	ProcessNextWorkItem() bool
	RunWorker()
}

func NewEventBuilder(kubeClient *kubeClient.KubeClient,
	logger *log.Logger,
) (Builder, error) {
	builder := &builder{
		kubeClient: kubeClient,
		workqueue:  initWorkqueue(),
		recorder:   initRecorder(kubeClient),
		logger:     logger,
	}

	return builder, nil
}

func initRecorder(kubeClient *kubeClient.KubeClient) record.EventRecorder {
	// Initliaze Event Broadcaster
	policyscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{

			Interface: kubeClient.GetEventsInterface("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: utils.EventSource})
	return recorder
}

func initWorkqueue() workqueue.RateLimitingInterface {
	return workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), utils.EventWorkQueueName)
}

func (b *builder) SetController(controller controllerinternalinterfaces.PolicyGetter) {
	b.controller = controller
	b.policySynced = controller.GetCacheInformerSync()
}

func (b *builder) AddEvent(info utils.EventInfo) {
	b.workqueue.Add(info)
}

// Run : Initialize the worker routines to process the event creation
func (b *builder) Run(threadiness int, stopCh <-chan struct{}) error {
	if b.controller == nil {
		return errors.New("Controller has not be set")
	}
	defer utilruntime.HandleCrash()
	defer b.workqueue.ShutDown()
	log.Println("Starting violation builder")

	fmt.Println(("Wait for informer cache to sync"))
	if ok := cache.WaitForCacheSync(stopCh, b.policySynced); !ok {
		fmt.Println("Unable to sync the cache")
	}
	log.Println("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(b.RunWorker, time.Second, stopCh)
	}
	log.Println("Started workers")
	<-stopCh
	log.Println("Shutting down workers")
	return nil

}

func (b *builder) RunWorker() {
	for b.ProcessNextWorkItem() {
	}
}

func (b *builder) ProcessNextWorkItem() bool {
	obj, shutdown := b.workqueue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer b.workqueue.Done(obj)
		var key utils.EventInfo
		var ok bool
		if key, ok = obj.(utils.EventInfo); !ok {
			b.workqueue.Forget(obj)
			log.Printf("Expecting type info by got %v", obj)
			return nil
		}

		// Run the syncHandler, passing the resource and the policy
		if err := b.SyncHandler(key); err != nil {
			b.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s' : %s, requeuing event creation request", key.Resource, err.Error())
		}

		return nil
	}(obj)

	if err != nil {
		log.Println((err))
	}
	return true
}

func (b *builder) SyncHandler(key utils.EventInfo) error {
	var resource runtime.Object
	var err error
	switch key.Kind {
	case "Policy":
		resource, err = b.controller.GetPolicy(key.Resource)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for policy %s, will retry ", key.Resource))
			return err
		}
	default:
		resource, err = b.kubeClient.GetResource(key.Kind, key.Resource)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for resource %s, will retry ", key.Resource))
			return err
		}
	}
	b.recorder.Event(resource, v1.EventTypeNormal, key.Reason, key.Message)
	return nil
}
