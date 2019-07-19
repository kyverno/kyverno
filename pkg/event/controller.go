package event

import (
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	policyscheme "github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	v1alpha1 "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	client       *client.Client
	policyLister v1alpha1.PolicyLister
	queue        workqueue.RateLimitingInterface
	recorder     record.EventRecorder
}

//Generator to generate event
type Generator interface {
	Add(infoList ...*Info)
}

//Controller  api
type Controller interface {
	Generator
	Run(stopCh <-chan struct{})
	Stop()
}

//NewEventController to generate a new event controller
func NewEventController(client *client.Client,
	shareInformer sharedinformer.PolicyInformer) Controller {

	return &controller{
		client:       client,
		policyLister: shareInformer.GetLister(),
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), eventWorkQueueName),
		recorder:     initRecorder(client),
	}
}

func initRecorder(client *client.Client) record.EventRecorder {
	// Initliaze Event Broadcaster
	err := policyscheme.AddToScheme(scheme.Scheme)
	if err != nil {
		glog.Error(err)
		return nil
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventInterface, err := client.GetEventsInterface()
	if err != nil {
		glog.Error(err) // TODO: add more specific error
		return nil
	}
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: eventInterface})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: eventSource})
	return recorder
}

func (c *controller) Add(infos ...*Info) {
	for _, info := range infos {
		c.queue.Add(*info)
	}
}

func (c *controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for i := 0; i < eventWorkerThreadCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	glog.Info("Started eventbuilder controller workers")
}

func (c *controller) Stop() {
	defer c.queue.ShutDown()
	glog.Info("Shutting down eventbuilder controller workers")
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < workQueueRetryLimit {
		glog.Warningf("Error syncing events %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	glog.Error(err)
	glog.Warningf("Dropping the key out of the queue: %v", err)
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
			glog.Warningf("Expecting type info by got %v\n", obj)
			return nil
		}
		err := c.syncHandler(key)
		c.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		glog.Error(err)
		return true
	}
	return true
}

func (c *controller) syncHandler(key Info) error {
	var robj runtime.Object
	var err error

	switch key.Kind {
	case "Policy":
		//TODO: policy is clustered resource so wont need namespace
		robj, err = c.policyLister.Get(key.Name)
		if err != nil {
			glog.Errorf("Error creating event: unable to get policy %s, will retry ", key.Name)
			return err
		}
	default:
		robj, err = c.client.GetResource(key.Kind, key.Namespace, key.Name)
		if err != nil {
			glog.Errorf("Error creating event: unable to get resource %s, %s, will retry ", key.Kind, key.Namespace+"/"+key.Name)
			return err
		}
	}

	if key.Reason == PolicyApplied.String() {
		c.recorder.Event(robj, v1.EventTypeNormal, key.Reason, key.Message)
	} else {
		c.recorder.Event(robj, v1.EventTypeWarning, key.Reason, key.Message)
	}
	return nil
}

//NewEvent returns a new event
func NewEvent(rkind string, rnamespace string, rname string, reason Reason, message MsgKey, args ...interface{}) *Info {
	msgText, err := getEventMsg(message, args...)
	if err != nil {
		glog.Error(err)
	}
	return &Info{
		Kind:      rkind,
		Name:      rname,
		Namespace: rnamespace,
		Reason:    reason.String(),
		Message:   msgText,
	}
}
