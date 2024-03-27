package event

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/tools/record/util"
	"k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
)

const (
	Workers             = 3
	ControllerName      = "kyverno-events"
	workQueueRetryLimit = 3

	finishTime  = 6 * time.Minute
	refreshTime = 30 * time.Minute
)

// Interface to generate event
type Interface interface {
	Add(infoList ...Info)
}

// controller generate events
type controller struct {
	logger               logr.Logger
	eventsClient         v1.EventsV1Interface
	omitEvents           sets.Set[string]
	queue                workqueue.RateLimitingInterface
	eventCache           EventCache
	mu                   sync.Mutex
	clock                clock.Clock
	hostname             string
	droppedEventsCounter metric.Int64Counter
}

// NewEventGenerator to generate a new event controller
func NewEventGenerator(eventsClient v1.EventsV1Interface, logger logr.Logger, omitEvents ...string) *controller {
	clock := clock.RealClock{}
	hostname, _ := os.Hostname()
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	droppedEventsCounter, err := meter.Int64Counter(
		"kyverno_events_dropped",
		metric.WithDescription("can be used to track the number of events dropped by the event generator"),
	)
	if err != nil {
		logger.Error(err, "failed to register metric kyverno_events_dropped")
	}
	return &controller{
		logger:               logger,
		eventsClient:         eventsClient,
		omitEvents:           sets.New(omitEvents...),
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		eventCache:           EventCache{},
		clock:                clock,
		hostname:             hostname,
		droppedEventsCounter: droppedEventsCounter,
	}
}

// Add queues an event for generation
func (gen *controller) Add(infos ...Info) {
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
func (gen *controller) Run(ctx context.Context, workers int) {
	logger := gen.logger
	logger.Info("start")
	defer logger.Info("terminated")
	defer utilruntime.HandleCrash()

	go wait.UntilWithContext(ctx, gen.refreshExistingEventSeries, refreshTime)
	go wait.UntilWithContext(ctx, gen.finishSeries, finishTime)

	var waitGroup wait.Group
	for i := 0; i < workers; i++ {
		waitGroup.StartWithContext(ctx, func(ctx context.Context) {
			for gen.processNextWorkItem(ctx) {
			}
		})
	}
	<-ctx.Done()
	gen.queue.ShutDownWithDrain()
	waitGroup.Wait()
}

func (gen *controller) processNextWorkItem(ctx context.Context) bool {
	logger := gen.logger

	key, quit := gen.queue.Get()
	if quit {
		return false
	}
	defer gen.queue.Done(key)

	event, ok := key.(*eventsv1.Event)
	if !ok {
		logger.Error(nil, "failed to convert key to Info", "key", key)
		return true
	}

	event = gen.preProcess(event)
	_, retry := gen.recordEvent(ctx, event)

	if retry {
		if gen.queue.NumRequeues(key) < workQueueRetryLimit {
			gen.queue.AddRateLimited(key)
			return true
		}
		gen.droppedEventsCounter.Add(ctx, 1)
		logger.Error(fmt.Errorf("failed to create event after %d retries", workQueueRetryLimit), "dropping event", "key", key)
	}

	gen.queue.Forget(key)

	return true
}

func (gen *controller) refreshExistingEventSeries(ctx context.Context) {
	gen.mu.Lock()
	defer gen.mu.Unlock()
	for isomorphicKey, event := range gen.eventCache {
		if event.Series != nil {
			if recordedEvent, retry := gen.recordEvent(ctx, event); !retry {
				if recordedEvent != nil {
					gen.eventCache[isomorphicKey] = recordedEvent
				}
			}
		}
	}
}

func (gen *controller) finishSeries(ctx context.Context) {
	gen.mu.Lock()
	defer gen.mu.Unlock()
	for isomorphicKey, event := range gen.eventCache {
		eventSerie := event.Series
		if eventSerie != nil {
			if eventSerie.LastObservedTime.Time.Before(time.Now().Add(-finishTime)) {
				if _, retry := gen.recordEvent(ctx, event); !retry {
					delete(gen.eventCache, isomorphicKey)
				}
			}
		} else if event.EventTime.Time.Before(time.Now().Add(-finishTime)) {
			delete(gen.eventCache, isomorphicKey)
		}
	}
}

func (gen *controller) preProcess(event *eventsv1.Event) *eventsv1.Event {
	eventKey := getKey(event)
	isomorphicEvent, isIsomorphic := gen.eventCache[eventKey]
	if isIsomorphic {
		if isomorphicEvent.Series != nil {
			isomorphicEvent.Series.Count++
			isomorphicEvent.Series.LastObservedTime = metav1.MicroTime{Time: gen.clock.Now()}
			return nil
		}
		isomorphicEvent.Series = &eventsv1.EventSeries{
			Count:            2,
			LastObservedTime: metav1.MicroTime{Time: gen.clock.Now()},
		}

		return isomorphicEvent.DeepCopy()
	}

	gen.eventCache[eventKey] = event

	return event.DeepCopy()
}

func (gen *controller) recordEvent(ctx context.Context, event *eventsv1.Event) (*eventsv1.Event, bool) {
	logger := gen.logger

	var newEvent *eventsv1.Event
	var err error

	isEventSeries := event.Series != nil
	if isEventSeries {
		patch, patchBytesErr := createPatchBytesForSeries(event)
		if patchBytesErr != nil {
			logger.Error(patchBytesErr, "Unable to calculate diff, no merge is possible")
			return nil, false
		}
		newEvent, err = gen.eventsClient.Events(event.Namespace).Patch(ctx, event.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	}

	if !isEventSeries || (isEventSeries && util.IsKeyNotFoundError(err)) {
		event.ResourceVersion = ""
		newEvent, err = gen.eventsClient.Events(event.Namespace).Create(ctx, event, metav1.CreateOptions{})
	}
	if err == nil {
		return newEvent, false
	}

	return nil, true
}

func createPatchBytesForSeries(event *eventsv1.Event) ([]byte, error) {
	oldEvent := event.DeepCopy()
	oldEvent.Series = nil
	oldData, err := json.Marshal(oldEvent)
	if err != nil {
		return nil, err
	}
	newData, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return strategicpatch.CreateTwoWayMergePatch(oldData, newData, eventsv1.Event{})
}

func (gen *controller) emitEvent(key Info) {
	logger := gen.logger
	eventType := corev1.EventTypeWarning
	if key.Type != "" {
		eventType = key.Type
	} else if key.Reason == PolicyApplied || key.Reason == PolicySkipped {
		eventType = corev1.EventTypeNormal
	}

	timestamp := metav1.MicroTime{Time: time.Now()}
	refRegarding, err := reference.GetReference(scheme.Scheme, &key.Regarding)
	if err != nil {
		logger.Error(err, "Could not construct reference, will not report event", "object", &key.Regarding, "eventType", eventType, "reason", string(key.Reason), "message", key.Message)
		return
	}

	var refRelated *corev1.ObjectReference
	if key.Related != nil {
		refRelated, err = reference.GetReference(scheme.Scheme, key.Related)
		if err != nil {
			logger.V(9).Info("Could not construct reference", "object", key.Related, "err", err)
		}
	}
	if !util.ValidateEventType(eventType) {
		logger.Error(nil, "Unsupported event type", "eventType", eventType)
		return
	}

	reportingController := string(key.Source)
	reportingInstance := reportingController + "-" + gen.hostname

	t := metav1.Time{Time: gen.clock.Now()}
	namespace := refRegarding.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	event := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", refRegarding.Name, t.UnixNano()),
			Namespace: namespace,
		},
		EventTime:           timestamp,
		Series:              nil,
		ReportingController: reportingController,
		ReportingInstance:   reportingInstance,
		Action:              string(key.Action),
		Reason:              string(key.Reason),
		Regarding:           *refRegarding,
		Related:             refRelated,
		Note:                key.Message,
		Type:                eventType,
	}

	gen.queue.Add(event)
}
