package event

import (
	"time"

	"github.com/golang/glog"

	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

//Generator generate events
type Generator struct {
	client   *client.Client
	pLister  kyvernolister.ClusterPolicyLister
	queue    workqueue.RateLimitingInterface
	recorder record.EventRecorder
}

//Interface to generate event
type Interface interface {
	Add(infoList ...Info)
}

//NewEventGenerator to generate a new event controller
func NewEventGenerator(client *client.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer) *Generator {

	gen := Generator{
		client:   client,
		pLister:  pInformer.Lister(),
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), eventWorkQueueName),
		recorder: initRecorder(client),
	}

	return &gen
}

func initRecorder(client *client.Client) record.EventRecorder {
	// Initliaze Event Broadcaster
	err := scheme.AddToScheme(scheme.Scheme)
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

//Add queues an event for generation
func (gen *Generator) Add(infos ...Info) {
	for _, info := range infos {
		if info.Name == "" {
			// dont create event for resources with generateName
			// as the name is not generated yet
			glog.V(4).Infof("recieved info %v, not creating an event as the resource has not been assigned a name yet", info)
			continue
		}
		gen.queue.Add(info)
	}
}

// Run begins generator
func (gen *Generator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	glog.Info("Starting event generator")
	defer glog.Info("Shutting down event generator")

	for i := 0; i < workers; i++ {
		go wait.Until(gen.runWorker, time.Second, stopCh)
	}
	<-stopCh
}

func (gen *Generator) runWorker() {
	for gen.processNextWorkItem() {
	}
}

func (gen *Generator) handleErr(err error, key interface{}) {
	if err == nil {
		gen.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		glog.Warningf("Error syncing events %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		gen.queue.AddRateLimited(key)
		return
	}
	gen.queue.Forget(key)
	glog.Error(err)
	glog.Warningf("Dropping the key out of the queue: %v", err)
}

func (gen *Generator) processNextWorkItem() bool {
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer gen.queue.Done(obj)
		var key Info
		var ok bool

		if key, ok = obj.(Info); !ok {
			gen.queue.Forget(obj)
			glog.Warningf("Expecting type info by got %v\n", obj)
			return nil
		}
		err := gen.syncHandler(key)
		gen.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		glog.Error(err)
		return true
	}
	return true
}

func (gen *Generator) syncHandler(key Info) error {
	var robj runtime.Object
	var err error
	switch key.Kind {
	case "Policy":
		//TODO: policy is clustered resource so wont need namespace
		robj, err = gen.pLister.Get(key.Name)
		if err != nil {
			glog.Errorf("Error creating event: unable to get policy %s, will retry ", key.Name)
			return err
		}
	default:
		robj, err = gen.client.GetResource(key.Kind, key.Namespace, key.Name)
		if err != nil {
			glog.Errorf("Error creating event: unable to get resource %s, %s, will retry ", key.Kind, key.Namespace+"/"+key.Name)
			return err
		}
	}

	if key.Reason == PolicyApplied.String() {
		gen.recorder.Event(robj, v1.EventTypeNormal, key.Reason, key.Message)
	} else {
		gen.recorder.Event(robj, v1.EventTypeWarning, key.Reason, key.Message)
	}
	return nil
}

//TODO: check if we need this ?
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

func NewEventNew(
	rkind,
	rapiVersion,
	rnamespace,
	rname,
	reason string,
	message MsgKey,
	args ...interface{}) Info {
	msgText, err := getEventMsg(message, args...)
	if err != nil {
		glog.Error(err)
	}
	return Info{
		Kind:      rkind,
		Name:      rname,
		Namespace: rnamespace,
		Reason:    reason,
		Message:   msgText,
	}
}
