package event

import (
	"fmt"
	"log"
	"time"

	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/pkg/client/clientset/versioned/scheme"
	policyscheme "github.com/nirmata/kube-policy/pkg/client/clientset/versioned/scheme"
	policylister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type eventController struct {
	kubeClient   *kubeClient.KubeClient
	policyLister policylister.PolicyLister
	queue        workqueue.RateLimitingInterface
	recorder     record.EventRecorder
	logger       *log.Logger
}

// EventGenertor to generate event
type EventGenerator interface {
	Add(kind string, resource string, reason Reason, message EventMsg, args ...interface{})
}
type EventController interface {
	EventGenerator
	Run(stopCh <-chan struct{}) error
}

func NewEventController(kubeClient *kubeClient.KubeClient,
	policyLister policylister.PolicyLister,
	logger *log.Logger) EventController {
	controller := &eventController{
		kubeClient:   kubeClient,
		policyLister: policyLister,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), eventWorkQueueName),
		recorder:     initRecorder(kubeClient),
		logger:       logger,
	}
	return controller
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
		v1.EventSource{Component: eventSource})
	return recorder
}

func (eb *eventController) Add(kind string, resource string, reason Reason, message EventMsg, args ...interface{}) {
	eb.queue.Add(eb.newEvent(
		kind,
		resource,
		reason,
		message,
	))
}

// Run : Initialize the worker routines to process the event creation
func (eb *eventController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer eb.queue.ShutDown()

	log.Println("starting eventbuilder controller")

	log.Println("Starting eventbuilder controller workers")
	for i := 0; i < eventWorkerThreadCount; i++ {
		go wait.Until(eb.runWorker, time.Second, stopCh)
	}
	log.Println("Started eventbuilder controller workers")
	<-stopCh
	log.Println("Shutting down eventbuilder controller workers")
	return nil
}

func (eb *eventController) runWorker() {
	for eb.processNextWorkItem() {
	}
}

func (eb *eventController) processNextWorkItem() bool {
	obj, shutdown := eb.queue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer eb.queue.Done(obj)
		var key eventInfo
		var ok bool
		if key, ok = obj.(eventInfo); !ok {
			eb.queue.Forget(obj)
			log.Printf("Expecting type info by got %v", obj)
			return nil
		}
		// Run the syncHandler, passing the resource and the policy
		if err := eb.SyncHandler(key); err != nil {
			eb.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s' : %s, requeuing event creation request", key.Resource, err.Error())
		}
		return nil
	}(obj)

	if err != nil {
		log.Println((err))
	}
	return true
}

func (eb *eventController) SyncHandler(key eventInfo) error {
	var resource runtime.Object
	var err error
	switch key.Kind {
	case "Policy":
		namespace, name, err := cache.SplitMetaNamespaceKey(key.Resource)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to extract namespace and name for %s", key.Resource))
			return err
		}
		resource, err = eb.policyLister.Policies(namespace).Get(name)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for policy %s, will retry ", key.Resource))
			return err
		}
	default:
		resource, err = eb.kubeClient.GetResource(key.Kind, key.Resource)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for resource %s, will retry ", key.Resource))
			return err
		}
	}
	eb.recorder.Event(resource, v1.EventTypeNormal, key.Reason, key.Message)
	return nil
}

type eventInfo struct {
	Kind     string
	Resource string
	Reason   string
	Message  string
}

func (eb *eventController) newEvent(kind string, resource string, reason Reason, message EventMsg, args ...interface{}) eventInfo {
	msgText, err := getEventMsg(message, args)
	if err != nil {
		utilruntime.HandleError(err)
	}
	return eventInfo{
		Kind:     kind,
		Resource: resource,
		Reason:   reason.String(),
		Message:  msgText,
	}
}
