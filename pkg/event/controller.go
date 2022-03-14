package event

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	v1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

//Generator generate events
type Generator struct {
	client *client.Client
	// list/get cluster policy
	cpLister kyvernolister.ClusterPolicyLister
	// returns true if the cluster policy store has been synced at least once
	cpSynced cache.InformerSynced
	// list/get policy
	pLister kyvernolister.PolicyLister
	// returns true if the policy store has been synced at least once
	pSynced cache.InformerSynced
	// queue to store event generation requests
	queue workqueue.RateLimitingInterface
	// events generated at policy controller
	policyCtrRecorder record.EventRecorder
	// events generated at admission control
	admissionCtrRecorder record.EventRecorder
	// events generated at namespaced policy controller to process 'generate' rule
	genPolicyRecorder record.EventRecorder
	log               logr.Logger
}

//Interface to generate event
type Interface interface {
	Add(infoList ...Info)
}

//NewEventGenerator to generate a new event controller
func NewEventGenerator(client *client.Client, cpInformer kyvernoinformer.ClusterPolicyInformer, pInformer kyvernoinformer.PolicyInformer, log logr.Logger) *Generator {

	gen := Generator{
		client:               client,
		cpLister:             cpInformer.Lister(),
		cpSynced:             cpInformer.Informer().HasSynced,
		pLister:              pInformer.Lister(),
		pSynced:              pInformer.Informer().HasSynced,
		queue:                workqueue.NewNamedRateLimitingQueue(rateLimiter(), eventWorkQueueName),
		policyCtrRecorder:    initRecorder(client, PolicyController, log),
		admissionCtrRecorder: initRecorder(client, AdmissionController, log),
		genPolicyRecorder:    initRecorder(client, GeneratePolicyController, log),
		log:                  log,
	}
	return &gen
}

func rateLimiter() workqueue.RateLimiter {
	return workqueue.DefaultItemBasedRateLimiter()
}

func initRecorder(client *client.Client, eventSource Source, log logr.Logger) record.EventRecorder {
	// Initliaze Event Broadcaster
	err := scheme.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "failed to add to scheme")
		return nil
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.V(5).Infof)
	eventInterface, err := client.GetEventsInterface()
	if err != nil {
		log.Error(err, "failed to get event interface for logging")
		return nil
	}
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: eventInterface})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: eventSource.String()})
	return recorder
}

//Add queues an event for generation
func (gen *Generator) Add(infos ...Info) {
	logger := gen.log
	for _, info := range infos {
		if info.Name == "" {
			// dont create event for resources with generateName
			// as the name is not generated yet
			logger.V(4).Info("not creating an event as the resource has not been assigned a name yet", "kind", info.Kind, "name", info.Name, "namespace", info.Namespace)
			continue
		}
		gen.queue.Add(info)
	}
}

// Run begins generator
func (gen *Generator) Run(workers int, stopCh <-chan struct{}) {
	logger := gen.log
	defer utilruntime.HandleCrash()

	logger.Info("start")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, gen.cpSynced, gen.pSynced) {
		logger.Info("failed to sync informer cache")
	}

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
	logger := gen.log
	if err == nil {
		gen.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.V(4).Info("retrying event generation", "key", key, "reason", err.Error())
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		gen.queue.AddRateLimited(key)
		return
	}

	gen.queue.Forget(key)
	if !errors.IsNotFound(err) {
		logger.Error(err, "failed to generate event", "key", key)
	}
}

func (gen *Generator) processNextWorkItem() bool {
	logger := gen.log
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
			logger.Info("Incorrect type; expected type 'info'", "obj", obj)
			return nil
		}
		err := gen.syncHandler(key)
		gen.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		logger.Error(err, "failed to process next work item")
		return true
	}
	return true
}

func (gen *Generator) syncHandler(key Info) error {
	logger := gen.log
	var robj runtime.Object
	var err error
	switch key.Kind {
	case "ClusterPolicy":
		robj, err = gen.cpLister.Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get cluster policy", "name", key.Name)
			return err
		}
	case "Policy":
		robj, err = gen.pLister.Policies(key.Namespace).Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get policy", "name", key.Name)
			return err
		}
	default:
		robj, err = gen.client.GetResource("", key.Kind, key.Namespace, key.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "failed to get resource", "kind", key.Kind, "name", key.Name, "namespace", key.Namespace)
			}
			return err
		}
	}

	// set the event type based on reason
	eventType := v1.EventTypeWarning
	if key.Reason == PolicyApplied.String() {
		eventType = v1.EventTypeNormal
	}

	// based on the source of event generation, use different event recorders
	switch key.Source {
	case AdmissionController:
		gen.admissionCtrRecorder.Event(robj, eventType, key.Reason, key.Message)
	case PolicyController:
		gen.policyCtrRecorder.Event(robj, eventType, key.Reason, key.Message)
	case GeneratePolicyController:
		gen.genPolicyRecorder.Event(robj, eventType, key.Reason, key.Message)
	default:
		logger.Info("info.source not defined for the request")
	}
	return nil
}

//NewEvent builds a event creation request
func NewEvent(
	log logr.Logger,
	rkind,
	rapiVersion,
	rnamespace,
	rname,
	reason string,
	source Source,
	message MsgKey,
	args ...interface{}) Info {
	msgText, err := getEventMsg(message, args...)
	if err != nil {
		log.Error(err, "failed to get event message")
	}
	return Info{
		Kind:      rkind,
		Name:      rname,
		Namespace: rnamespace,
		Reason:    reason,
		Source:    source,
		Message:   msgText,
	}
}
