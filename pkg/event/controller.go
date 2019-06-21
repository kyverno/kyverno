package event

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	policyscheme "github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	v1alpha1 "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/result"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
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
	client       *client.Client
	policyLister v1alpha1.PolicyLister
	queue        workqueue.RateLimitingInterface
	recorder     record.EventRecorder
}

//Generator to generate event
type Generator interface {
	Add(infoList []*Info)
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

	controller := &controller{
		client:       client,
		policyLister: shareInformer.GetLister(),
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), eventWorkQueueName),
		recorder:     initRecorder(client),
	}
	return controller
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

func (c *controller) Add(infoList []*Info) {
	for _, info := range infoList {
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
		// Run the syncHandler, passing the resource and the policy
		if err := c.SyncHandler(key); err != nil {
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s' : %s, requeuing event creation request", key.Resource, err.Error())
		}
		return nil
	}(obj)

	if err != nil {
		glog.Warning(err)
	}
	return true
}

func (c *controller) SyncHandler(key Info) error {
	var resource runtime.Object
	var err error

	switch key.Kind {
	case "Policy":
		//TODO: policy is clustered resource so wont need namespace
		resource, err = c.policyLister.Get(key.Resource)
		if err != nil {
			glog.Errorf("Unable to create event for policy %s, will retry ", key.Resource)
			return err
		}
	default:
		namespace, name, err := cache.SplitMetaNamespaceKey(key.Resource)
		if err != nil {
			glog.Errorf("Invalid resource key: %s", key.Resource)
			return err
		}
		rName := c.client.DiscoveryClient.GetGVRFromKind(key.Kind).Resource
		resource, err = c.client.GetResource(rName, namespace, name)
		if err != nil {
			glog.Errorf("Unable to create event for resource %s, will retry ", key.Resource)
			return err
		}
	}

	glog.Infof("Creating event for resource %s: %s\n", key.Kind, key.Resource)
	c.recorder.Event(resource, v1.EventTypeNormal, key.Reason, key.Message)
	return nil
}

//NewEvent returns a new event
func NewEvent(kind string, resource string, reason result.Reason, message MsgKey, args ...interface{}) *Info {
	msgText, err := getEventMsg(message, args...)
	if err != nil {
		glog.Errorf("Failed to get event message text, err: %v\n", err)
	}

	return &Info{
		Kind:     kind,
		Resource: resource,
		Reason:   reason.String(),
		Message:  msgText,
	}
}

// NewEventsFromResultOnResourceCreation create event info list from result
// this should be called on resource creation
func NewEventsFromResultOnResourceCreation(kind string, resource string, rslt result.Result) []*Info {
	var infoList []*Info
	switch rslt.GetReason() {
	case result.Success:
		// create event for policy
		infoList = append(infoList, NewEvent(policyKind, rslt.Name(), result.Success, SPolicyApply, rslt.Name(), resource))
		// create event for resource
		infoList = append(infoList, NewEvent(kind, resource, result.Success, SPolicyApply, rslt.Name(), resource))

		glog.V(3).Infof("Success events info prepared for %s/%s and %s/%s\n", policyKind, rslt.Name(), kind, resource)

		return infoList
	case result.Failed:
		var ruleNames []string
		results := rslt.GetChildren()
		for _, r := range results {
			if r.GetReason() != result.Success {
				ruleNames = append(ruleNames, r.Name())
			}
		}

		rn := strings.Join(ruleNames, ",")
		info := NewEvent(policyKind, rslt.Name(), result.Failed, FPolicyApplyBlockCreate, resource, rn)
		glog.V(3).Infof("Rule(s) %s of policy %s blocked resource creation\n", rn, rslt.Name())
		glog.V(4).Infof("Policy block creation event info %v\n", *info)

		return append(infoList, info)
	}
	return nil
}

// NewEventsFromResultOnPolicyOperation create policyViolaton event
// this should be called on policy changes
// i.e. policy update / creation
func NewEventsFromResultOnPolicyOperation(kind string, resource string, rslt result.Result) []*Info {
	if rslt.GetReason() != result.Violation {
		glog.V(3).Infof("Return on result reason %s\n", rslt.GetReason())
		return nil
	}

	var infoList []*Info
	var ruleNames []string

	results := rslt.GetChildren()
	for _, r := range results {
		// add events to resource only on failure
		if r.GetReason() == result.Failed {
			ruleNames = append(ruleNames, r.Name())
			// TODO: change TBD to violation ID below
			info := NewEvent(kind, resource, result.Violation, FProcessRule, r.Name(), rslt.Name(), "TBD")
			infoList = append(infoList, info)
			glog.V(3).Infof("Policy violation event prepared for rule %s of policy %s\n", r.Name(), rslt.Name())
			glog.V(4).Infof("Policy violation event info %v\n", *info)
		}
	}

	// create policyViolation event for policy
	// TODO: change TBD to violation ID below
	info := NewEvent(policyKind, rslt.Name(), result.Violation, FProcessPolicy, resource, strings.Join(ruleNames, ","), "TBD")
	glog.V(3).Infof("Event prepared for policy %s\n", rslt.Name())
	glog.V(4).Infof("Fail to process policy event info: %v\n", *info)

	return append(infoList, info)
}
