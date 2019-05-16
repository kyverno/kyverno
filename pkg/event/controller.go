package event

import (
	"fmt"
	"log"
	"os"
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

type controller struct {
	kubeClient   *kubeClient.KubeClient
	policyLister policylister.PolicyLister
	queue        workqueue.RateLimitingInterface
	recorder     record.EventRecorder
	logger       *log.Logger
}

//Generator to generate event
type Generator interface {
	Add(info Info)
}

//Controller  api
type Controller interface {
	Generator
	Run(stopCh <-chan struct{})
}

//NewEventController to generate a new event controller
func NewEventController(kubeClient *kubeClient.KubeClient,
	policyLister policylister.PolicyLister,
	logger *log.Logger) Controller {

	if logger == nil {
		logger = log.New(os.Stdout, "Event Controller:  ", log.LstdFlags)
	}

	controller := &controller{
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
			Interface: kubeClient.GetEvents("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: eventSource})
	return recorder
}

func (c *controller) Add(info Info) {
	c.queue.Add(info)
}

func (c *controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	for i := 0; i < eventWorkerThreadCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	c.logger.Println("Started eventbuilder controller")
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer c.queue.Done(obj)
		var key Info
		var ok bool
		if key, ok = obj.(Info); !ok {
			c.queue.Forget(obj)
			c.logger.Printf("Expecting type info by got %v\n", obj)
			return nil
		}
		// Run the syncHandler, passing the resource and the policy
		if err := c.SyncHandler(key); err != nil {
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s' : %s, requeuing event creation request", key.Resource, err.Error())
		}
		return nil
	}(obj)

	if err != nil {
		log.Println(err)
	}
	return true
}

func (c *controller) SyncHandler(key Info) error {
	var resource runtime.Object
	var err error
	switch key.Kind {
	case "Policy":
		namespace, name, err := cache.SplitMetaNamespaceKey(key.Resource)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to extract namespace and name for %s", key.Resource))
			return err
		}
		resource, err = c.policyLister.Policies(namespace).Get(name)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for policy %s, will retry ", key.Resource))
			return err
		}
	default:
		resource, err = c.kubeClient.GetResource(key.Kind, key.Resource)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for resource %s, will retry ", key.Resource))
			return err
		}
	}
	c.recorder.Event(resource, v1.EventTypeNormal, key.Reason, key.Message)
	return nil
}

//NewEvent returns a new event
func NewEvent(kind string, resource string, reason Reason, message MsgKey, args ...interface{}) Info {
	msgText, err := getEventMsg(message, args)
	if err != nil {
		utilruntime.HandleError(err)
	}
	return Info{
		Kind:     kind,
		Resource: resource,
		Reason:   reason.String(),
		Message:  msgText,
	}
}
