package event

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	Workers             = 3
	CleanupWorkers      = 3
	eventWorkQueueName  = "kyverno-events"
	workQueueRetryLimit = 3
)

// generator generate events
type generator struct {
	// broadcaster
	broadcaster events.EventBroadcaster

	// recorders
	recorders map[Source]events.EventRecorder

	// config
	queue           workqueue.RateLimitingInterface
	maxQueuedEvents int
	omitEvents      sets.Set[string]
	logger          logr.Logger
}

// Interface to generate event
type Interface interface {
	Add(infoList ...Info)
}

// Controller interface to generate event
type Controller interface {
	Interface
	Run(context.Context, int, *sync.WaitGroup)
}

// NewEventGenerator to generate a new event controller
func NewEventGenerator(client dclient.Interface, maxQueuedEvents int, omitEvents []string, logger logr.Logger) Controller {
	return &generator{
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter(), eventWorkQueueName),
		broadcaster: events.NewBroadcaster(&events.EventSinkImpl{
			Interface: client.GetEventsInterface(),
		}),
		maxQueuedEvents: maxQueuedEvents,
		omitEvents:      sets.New(omitEvents...),
		logger:          logger,
	}
}

// NewEventGenerator to generate a new event cleanup controller
func NewEventCleanupGenerator(client dclient.Interface, maxQueuedEvents int, logger logr.Logger) Controller {
	return &generator{
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter(), eventWorkQueueName),
		broadcaster: events.NewBroadcaster(&events.EventSinkImpl{
			Interface: client.GetEventsInterface(),
		}),
		maxQueuedEvents: maxQueuedEvents,
		logger:          logger,
	}
}

// Add queues an event for generation
func (gen *generator) Add(infos ...Info) {
	logger := gen.logger
	logger.V(3).Info("generating events", "count", len(infos))
	if gen.maxQueuedEvents == 0 || gen.queue.Len() > gen.maxQueuedEvents {
		logger.V(2).Info("exceeds the event queue limit, dropping the event", "maxQueuedEvents", gen.maxQueuedEvents, "current size", gen.queue.Len())
		return
	}
	for _, info := range infos {
		// don't create event for resources with generateName as the name is not generated yet
		if info.Regarding.Name == "" {
			logger.V(3).Info("skipping event creation for resource without a name", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace)
			continue
		}
		if gen.omitEvents.Has(string(info.Reason)) {
			logger.V(6).Info("omitting event", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace, "reason", info.Reason)
			continue
		}
		gen.queue.Add(info)
		logger.V(6).Info("creating event", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace, "reason", info.Reason)
	}
}

// Run begins generator
func (gen *generator) Run(ctx context.Context, workers int, waitGroup *sync.WaitGroup) {
	logger := gen.logger
	logger.Info("start")
	defer logger.Info("terminated")
	defer utilruntime.HandleCrash()
	// TODO: we should probably wait workers exited before stopping recorders
	defer gen.stopRecorders()
	defer gen.queue.ShutDownWithDrain()
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
	if err := gen.broadcaster.StartRecordingToSinkWithContext(ctx); err != nil {
		return err
	}
	// TODO: we should probably wait workers exited before stopping recorders
	logger := klog.Background().V(int(0))
	if _, err := gen.broadcaster.StartLogging(logger); err != nil {
		return err
	}
	gen.recorders = map[Source]events.EventRecorder{
		PolicyController:         gen.broadcaster.NewRecorder(scheme.Scheme, string(PolicyController)),
		AdmissionController:      gen.broadcaster.NewRecorder(scheme.Scheme, string(AdmissionController)),
		GeneratePolicyController: gen.broadcaster.NewRecorder(scheme.Scheme, string(GeneratePolicyController)),
		MutateExistingController: gen.broadcaster.NewRecorder(scheme.Scheme, string(MutateExistingController)),
		CleanupController:        gen.broadcaster.NewRecorder(scheme.Scheme, string(CleanupController)),
	}
	return nil
}

func (gen *generator) stopRecorders() {
	gen.broadcaster.Shutdown()
}

func (gen *generator) runWorker(ctx context.Context) {
	for gen.processNextWorkItem() {
	}
}

func (gen *generator) processNextWorkItem() bool {
	if obj, quit := gen.queue.Get(); !quit {
		defer gen.queue.Done(obj)
		if key, ok := obj.(Info); ok {
			gen.handleErr(gen.syncHandler(key), obj)
		} else {
			gen.queue.Forget(obj)
			gen.logger.V(2).Info("Incorrect type; expected type 'info'", "obj", obj)
		}
		return true
	}
	return false
}

func (gen *generator) handleErr(err error, key interface{}) {
	logger := gen.logger
	if err == nil {
		gen.queue.Forget(key)
	} else {
		if gen.queue.NumRequeues(key) < workQueueRetryLimit {
			logger.V(4).Info("retrying event generation", "key", key, "reason", err.Error())
			gen.queue.AddRateLimited(key)
		} else {
			logger.Info("dropping event generation", "key", key, "reason", err.Error())
			gen.queue.Forget(key)
		}
	}
}

func (gen *generator) syncHandler(key Info) error {
	logger := gen.logger
	eventType := corev1.EventTypeWarning
	if key.Reason == PolicyApplied || key.Reason == PolicySkipped {
		eventType = corev1.EventTypeNormal
	}
	if recorder := gen.recorders[key.Source]; recorder != nil {
		logger.V(3).Info("creating the event", "source", key.Source, "type", eventType, "resource", key.Resource())
		recorder.Eventf(&key.Regarding, key.Related, eventType, string(key.Reason), string(key.Action), key.Message)
	} else {
		logger.Info("info.source not defined for the request")
	}
	return nil
}
