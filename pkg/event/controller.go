package event

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/events"
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
	Run(context.Context, int, *sync.WaitGroup)
}

// NewEventGenerator to generate a new event controller
func NewEventGenerator(client dclient.Interface, logger logr.Logger, omitEvents ...string) Controller {
	return &generator{
		broadcaster: events.NewBroadcaster(&events.EventSinkImpl{
			Interface: client.GetEventsInterface(),
		}),
		omitEvents: sets.New(omitEvents...),
		logger:     logger,
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
func (gen *generator) Run(ctx context.Context, workers int, waitGroup *sync.WaitGroup) {
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
	if err := gen.broadcaster.StartRecordingToSinkWithContext(ctx); err != nil {
		return err
	}
	logger := klog.Background().V(int(0))
	// TODO: logger watcher should be stopped
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
