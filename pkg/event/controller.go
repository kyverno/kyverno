package event

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	corev1 "k8s.io/api/core/v1"
	apieventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/tools/events"
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

	// client
	eventsClient eventsv1.EventsV1Interface

	// metrics
	droppedEventsCounter metric.Int64Counter

	// config
	omitEvents sets.Set[string]
	logger     logr.Logger
}

// Interface to generate event
type Interface interface {
	Add(infoList ...Info)
}

// Controller interface to generate event
type Controller interface {
	Interface
	Run(context.Context)
}

// NewEventGenerator to generate a new event controller
func NewEventGenerator(eventsClient eventsv1.EventsV1Interface, logger logr.Logger, omitEvents ...string) Controller {
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	droppedEventsCounter, err := meter.Int64Counter(
		"kyverno_events_dropped",
		metric.WithDescription("can be used to track the number of events dropped by the event generator"),
	)
	if err != nil {
		logger.Error(err, "failed to register metric kyverno_events_dropped")
	}
	return &generator{
		broadcaster: events.NewBroadcaster(&events.EventSinkImpl{
			Interface: eventsClient,
		}),
		eventsClient:         eventsClient,
		omitEvents:           sets.New(omitEvents...),
		logger:               logger,
		droppedEventsCounter: droppedEventsCounter,
	}
}

// Add queues an event for generation
func (gen *generator) Add(infos ...Info) {
	logger := gen.logger
	logger.V(3).Info("generating events", "count", len(infos))
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
		gen.emitEvent(info)
		logger.V(6).Info("creating event", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace, "reason", info.Reason)
	}
}

// Run begins generator
func (gen *generator) Run(ctx context.Context) {
	logger := gen.logger
	logger.Info("start")
	defer logger.Info("terminated")
	defer utilruntime.HandleCrash()
	defer gen.stopRecorders()
	defer logger.Info("shutting down...")
	if err := gen.startRecorders(ctx); err != nil {
		logger.Error(err, "failed to start recorders")
		return
	}
	<-ctx.Done()
}

func (gen *generator) startRecorders(ctx context.Context) error {
	eventHandler := func(obj runtime.Object) {
		event, ok := obj.(*apieventsv1.Event)
		if !ok {
			gen.logger.Error(nil, "unexpected type, expected eventsv1.Event")
			return
		}

		eventCopy := event.DeepCopy()
		eventCopy.ResourceVersion = ""
		_, err := gen.eventsClient.Events(event.Namespace).Create(ctx, eventCopy, metav1.CreateOptions{})
		if err != nil {
			gen.droppedEventsCounter.Add(ctx, 1)
			gen.logger.Error(err, "failed to create event")
		}
	}
	stopWatcher, err := gen.broadcaster.StartEventWatcher(eventHandler)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		stopWatcher()
	}()

	if _, err := gen.broadcaster.StartLogging(gen.logger); err != nil {
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

func (gen *generator) emitEvent(key Info) {
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
}
