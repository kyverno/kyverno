package event

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/workqueue"
)

const (
	eventWorkQueueName  = "kyverno-events"
	workQueueRetryLimit = 3
)

// generator generate events
type generator struct {
	client dclient.Interface
	// list/get cluster policy
	cpLister kyvernov1listers.ClusterPolicyLister
	// list/get policy
	pLister kyvernov1listers.PolicyLister
	// list/get cluster cleanup policy
	clustercleanuppolLister kyvernov2beta1listers.ClusterCleanupPolicyLister
	// list/get cleanup policy
	cleanuppolLister kyvernov2beta1listers.CleanupPolicyLister
	// queue to store event generation requests
	queue workqueue.RateLimitingInterface

	policyCtrBroadcaster      events.EventBroadcaster
	admissionCtrBroadcaster   events.EventBroadcaster
	genPolicyBroadcaster      events.EventBroadcaster
	mutateExistingBroadcaster events.EventBroadcaster
	cleanupPolicyBroadcaster  events.EventBroadcaster

	policyCtrRecorder      events.EventRecorder
	admissionCtrRecorder   events.EventRecorder
	genPolicyRecorder      events.EventRecorder
	mutateExistingRecorder events.EventRecorder
	cleanupPolicyRecorder  events.EventRecorder

	maxQueuedEvents int

	omitEvents []string

	log logr.Logger
}

// Controller interface to generate event
type Controller interface {
	Interface
	Run(context.Context, int, *sync.WaitGroup)
}

// Interface to generate event
type Interface interface {
	Add(infoList ...Info)
}

// NewEventGenerator to generate a new event controller
func NewEventGenerator(
	// source Source,
	client dclient.Interface,
	cpInformer kyvernov1informers.ClusterPolicyInformer,
	pInformer kyvernov1informers.PolicyInformer,
	maxQueuedEvents int,
	omitEvents []string,
	log logr.Logger,
) Controller {
	sink := newSink(client.GetEventsInterface())
	gen := generator{
		client:                    client,
		cpLister:                  cpInformer.Lister(),
		pLister:                   pInformer.Lister(),
		queue:                     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter(), eventWorkQueueName),
		policyCtrBroadcaster:      newBroadcaster(sink),
		admissionCtrBroadcaster:   newBroadcaster(sink),
		genPolicyBroadcaster:      newBroadcaster(sink),
		mutateExistingBroadcaster: newBroadcaster(sink),
		maxQueuedEvents:           maxQueuedEvents,
		omitEvents:                omitEvents,
		log:                       log,
	}
	return &gen
}

// NewEventGenerator to generate a new event cleanup controller
func NewEventCleanupGenerator(
	// source Source,
	client dclient.Interface,
	clustercleanuppolInformer kyvernov2beta1informers.ClusterCleanupPolicyInformer,
	cleanuppolInformer kyvernov2beta1informers.CleanupPolicyInformer,
	maxQueuedEvents int,
	log logr.Logger,
) Controller {
	sink := newSink(client.GetEventsInterface())
	gen := generator{
		client:                   client,
		clustercleanuppolLister:  clustercleanuppolInformer.Lister(),
		cleanuppolLister:         cleanuppolInformer.Lister(),
		queue:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter(), eventWorkQueueName),
		cleanupPolicyBroadcaster: newBroadcaster(sink),
		maxQueuedEvents:          maxQueuedEvents,
		log:                      log,
	}
	return &gen
}

// Add queues an event for generation
func (gen *generator) Add(infos ...Info) {
	logger := gen.log
	logger.V(3).Info("generating events", "count", len(infos))
	if gen.maxQueuedEvents == 0 || gen.queue.Len() > gen.maxQueuedEvents {
		logger.V(2).Info("exceeds the event queue limit, dropping the event", "maxQueuedEvents", gen.maxQueuedEvents, "current size", gen.queue.Len())
		return
	}
	for _, info := range infos {
		if info.Name == "" {
			// dont create event for resources with generateName
			// as the name is not generated yet
			logger.V(3).Info("skipping event creation for resource without a name", "kind", info.Kind, "name", info.Name, "namespace", info.Namespace)
			continue
		}
		shouldEmitEvent := true
		for _, eventReason := range gen.omitEvents {
			if info.Reason == Reason(eventReason) {
				shouldEmitEvent = false
				logger.V(6).Info("omitting event", "kind", info.Kind, "name", info.Name, "namespace", info.Namespace, "reason", info.Reason)
			}
		}
		if shouldEmitEvent {
			gen.queue.Add(info)
			logger.V(6).Info("creating event", "kind", info.Kind, "name", info.Name, "namespace", info.Namespace, "reason", info.Reason)
		}
	}
}

// Run begins generator
func (gen *generator) Run(ctx context.Context, workers int, waitGroup *sync.WaitGroup) {
	logger := gen.log
	logger.Info("start")
	defer logger.Info("terminated")
	defer utilruntime.HandleCrash()
	// TODO: we should probably wait workers exited before stopping recorders
	defer gen.stopRecorders()
	defer gen.queue.ShutDown()
	defer logger.Info("shutting down...")
	if err := gen.startRecorders(ctx); err != nil {
		logger.Error(err, "failed to start recorders")
		return
	}
	for i := 0; i < workers; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			wait.UntilWithContext(ctx, gen.runWorker, time.Second)
		}()
	}
	<-ctx.Done()
}

func (gen *generator) startRecorders(ctx context.Context) error {
	if gen.policyCtrBroadcaster != nil {
		policyCtrRecorder, err := startRecording(ctx, gen.policyCtrBroadcaster, PolicyController)
		if err != nil {
			return err
		}
		gen.policyCtrRecorder = policyCtrRecorder
	}
	if gen.admissionCtrBroadcaster != nil {
		admissionCtrRecorder, err := startRecording(ctx, gen.admissionCtrBroadcaster, AdmissionController)
		if err != nil {
			return err
		}
		gen.admissionCtrRecorder = admissionCtrRecorder
	}
	if gen.genPolicyBroadcaster != nil {
		genPolicyRecorder, err := startRecording(ctx, gen.genPolicyBroadcaster, GeneratePolicyController)
		if err != nil {
			return err
		}
		gen.genPolicyRecorder = genPolicyRecorder
	}
	if gen.mutateExistingBroadcaster != nil {
		mutateExistingRecorder, err := startRecording(ctx, gen.mutateExistingBroadcaster, MutateExistingController)
		if err != nil {
			return err
		}
		gen.mutateExistingRecorder = mutateExistingRecorder
	}
	if gen.cleanupPolicyBroadcaster != nil {
		cleanupPolicyRecorder, err := startRecording(ctx, gen.cleanupPolicyBroadcaster, CleanupController)
		if err != nil {
			return err
		}
		gen.cleanupPolicyRecorder = cleanupPolicyRecorder
	}
	return nil
}

func (gen *generator) stopRecorders() {
	if gen.policyCtrBroadcaster != nil {
		gen.policyCtrBroadcaster.Shutdown()
	}
	if gen.admissionCtrBroadcaster != nil {
		gen.admissionCtrBroadcaster.Shutdown()
	}
	if gen.genPolicyBroadcaster != nil {
		gen.genPolicyBroadcaster.Shutdown()
	}
	if gen.mutateExistingBroadcaster != nil {
		gen.mutateExistingBroadcaster.Shutdown()
	}
	if gen.cleanupPolicyBroadcaster != nil {
		gen.cleanupPolicyBroadcaster.Shutdown()
	}
}

func (gen *generator) runWorker(ctx context.Context) {
	for gen.processNextWorkItem() {
	}
}

func (gen *generator) handleErr(err error, key interface{}) {
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

func (gen *generator) processNextWorkItem() bool {
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}
	defer gen.queue.Done(obj)
	var key Info
	var ok bool
	if key, ok = obj.(Info); !ok {
		gen.queue.Forget(obj)
		gen.log.V(2).Info("Incorrect type; expected type 'info'", "obj", obj)
		return true
	}
	err := gen.syncHandler(key)
	gen.handleErr(err, obj)
	return true
}

func (gen *generator) syncHandler(key Info) error {
	logger := gen.log
	var regardingObj, relatedObj runtime.Object
	var err error
	switch key.Kind {
	case "ClusterPolicy":
		regardingObj, err = gen.cpLister.Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get cluster policy", "name", key.Name)
			return err
		}
	case "Policy":
		regardingObj, err = gen.pLister.Policies(key.Namespace).Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get policy", "name", key.Name)
			return err
		}
	case "ClusterCleanupPolicy":
		regardingObj, err = gen.clustercleanuppolLister.Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get cluster clean up policy", "name", key.Name)
			return err
		}
	case "CleanupPolicy":
		regardingObj, err = gen.cleanuppolLister.CleanupPolicies(key.Namespace).Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get cleanup policy", "name", key.Name)
			return err
		}
	default:
		regardingObj, err = gen.client.GetResource(context.TODO(), "", key.Kind, key.Namespace, key.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "failed to get resource", "kind", key.Kind, "name", key.Name, "namespace", key.Namespace)
				return nil
			}
			return err
		}
	}

	relatedObj = kubeutils.NewUnstructured(key.RelatedAPIVersion, key.RelatedKind, key.RelatedNamespace, key.RelatedName)

	// set the event type based on reason
	// if skip/pass, reason will be: NORMAL
	// else reason will be: WARNING
	eventType := corev1.EventTypeWarning
	if key.Reason == PolicyApplied || key.Reason == PolicySkipped {
		eventType = corev1.EventTypeNormal
	}

	logger.V(3).Info("creating the event", "source", key.Source, "type", eventType, "resource", key.Resource())
	// based on the source of event generation, use different event recorders
	switch key.Source {
	case AdmissionController:
		gen.admissionCtrRecorder.Eventf(regardingObj, relatedObj, eventType, string(key.Reason), string(key.Action), key.Message)
	case PolicyController:
		gen.policyCtrRecorder.Eventf(regardingObj, relatedObj, eventType, string(key.Reason), string(key.Action), key.Message)
	case GeneratePolicyController:
		gen.genPolicyRecorder.Eventf(regardingObj, relatedObj, eventType, string(key.Reason), string(key.Action), key.Message)
	case MutateExistingController:
		gen.mutateExistingRecorder.Eventf(regardingObj, relatedObj, eventType, string(key.Reason), string(key.Action), key.Message)
	case CleanupController:
		gen.cleanupPolicyRecorder.Eventf(regardingObj, relatedObj, eventType, string(key.Reason), string(key.Action), key.Message)
	default:
		logger.Info("info.source not defined for the request")
	}
	return nil
}
