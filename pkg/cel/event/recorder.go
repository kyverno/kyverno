package event

import (
	"context"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	kclock "k8s.io/utils/clock"

	"github.com/go-logr/logr"
	engine "github.com/kyverno/kyverno/pkg/cel/engine"
)

type EventRecorder interface {
	RecordPolicyEvent(ctx context.Context, source EventSource, engineResponse *engine.EngineResponse)
	Shutdown()
}

type eventRecorder struct {
	eventQueue          workqueue.TypedRateLimitingInterface[PolicyEvent]
	kubeEventRecorder   record.EventRecorder
	deduplicationCache  *sync.Map
	deduplicationWindow time.Duration
	filters             []EventFilterFunc
	clock               kclock.Clock
	logger              logr.Logger
	shutdownCancel      context.CancelFunc
	wg                  sync.WaitGroup
}

func NewEventRecorder(
	kubeEventRecorder record.EventRecorder,
	deduplicationWindow time.Duration,
	rateLimiterConfig workqueue.TypedRateLimiter[PolicyEvent],
	filters []EventFilterFunc,
	logger logr.Logger,
) EventRecorder {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	recorder := &eventRecorder{
		eventQueue:          workqueue.NewTypedRateLimitingQueue[PolicyEvent](rateLimiterConfig),
		kubeEventRecorder:   kubeEventRecorder,
		deduplicationCache:  &sync.Map{},
		deduplicationWindow: deduplicationWindow,
		filters:             filters,
		clock:               kclock.RealClock{},
		logger:              logger.WithName("event-recorder"),
		shutdownCancel:      shutdownCancel,
	}

	recorder.startEventProcessor(shutdownCtx)
	return recorder
}

func (r *eventRecorder) startEventProcessor(shutdownCtx context.Context) {
	r.wg.Add(1)
	go r.eventProcessor(shutdownCtx)
}

func (r *eventRecorder) Shutdown() {
	r.logger.Info("Shutting down event recorder...")
	r.shutdownCancel()
	r.eventQueue.ShutDown()
	r.wg.Wait()
	r.logger.Info("Event recorder shutdown complete.")
}

func (r *eventRecorder) RecordPolicyEvent(ctx context.Context, source EventSource, engineResponse *engine.EngineResponse) {
	if engineResponse == nil || engineResponse.Resource == nil {
		r.logger.V(4).Info("Cannot record event: EngineResponse or Resource is nil", "source", source)
		return
	}

	for _, policyResponse := range engineResponse.Policies {
		for _, ruleResponse := range policyResponse.Rules {
			event := PolicyEvent{
				Outcome:      determineEventOutcomeFromRuleStatus(ruleResponse.Status()),
				Source:       source,
				Actions:      strings.Join(convertActionsToStrings(policyResponse.Actions.UnsortedList()), ","),
				RuleResponse: NewRuleResponseDataFromEngineResponse(engineResponse, &policyResponse, &ruleResponse),
			}
			r.eventQueue.Add(event)
		}
	}
}

func (r *eventRecorder) eventProcessor(shutdownCtx context.Context) {
	defer r.wg.Done()
	logger := r.logger.WithName("event-processor")

	deduplicationTicker := time.NewTicker(r.deduplicationWindow / 2)
	defer deduplicationTicker.Stop()

	for {
		select {
		case <-shutdownCtx.Done():
			logger.Info("Event processor received shutdown signal, exiting.")
			return
		case <-deduplicationTicker.C:
			r.cleanupDeduplicationCache()
		default:
			event, shutdown := r.eventQueue.Get()
			if shutdown {
				logger.Info("Workqueue shutdown, exiting event processor.")
				return
			}

			if r.processEvent(event) {
				r.eventQueue.Forget(event)
			} else {
				r.eventQueue.AddRateLimited(event)
				logger.V(5).Info("Error processing event, re-enqueued for retry.", "outcome", event.Outcome, "source", event.Source, "rule", event.RuleName)
			}
			r.eventQueue.Done(event)
		}
	}
}

func (r *eventRecorder) processEvent(event PolicyEvent) bool {
	logger := r.logger.WithName("process-event")

	if r.isFiltered(event) {
		logger.V(5).Info("Event filtered, not processing.", eventDetailsLogFields(event)...)
		return true
	}

	eventKey := generateEventKey(event)
	if r.isDuplicate(eventKey) {
		logger.V(5).Info("Duplicate event detected, skipping.", eventDetailsLogFields(event)...)
		return true
	}

	r.recordEvent(event)
	return true
}

func (r *eventRecorder) isDuplicate(key EventKey) bool {
	_, found := r.deduplicationCache.Load(key)
	if found {
		return true
	}
	r.deduplicationCache.Store(key, r.clock.Now())
	return false
}

func (r *eventRecorder) cleanupDeduplicationCache() {
	r.deduplicationCache.Range(func(key, value interface{}) bool {
		if timestamp, ok := value.(time.Time); ok {
			if r.clock.Now().Sub(timestamp) > r.deduplicationWindow {
				r.deduplicationCache.Delete(key)
				r.logger.V(5).Info("Removed expired event signature from deduplication cache.", "key", key)
			}
		}
		return true
	})
}

func (r *eventRecorder) recordEvent(event PolicyEvent) {
	eventMsg := formatEventMessage(event)
	eventReason := string(event.Outcome)

	objRef := createObjectReferenceFromRuleResponse(event.RuleResponse)

	if objRef == nil {
		r.logger.V(4).Info("Could not create ObjectReference for event", eventDetailsLogFields(event)...)
	}

	if objRef != nil {
		eventType := getKubeEventType(event.Outcome)
		r.kubeEventRecorder.Eventf(objRef, eventType, eventReason, eventMsg)
	}

	logLevel := getLogLevel(event.Outcome)
	logEntry := r.logger.WithValues(eventDetailsLogFields(event)...)
	logEntry.V(logLevel).Info(eventMsg)
}

func FilterByPolicyName(policyName string) EventFilterFunc {
	return func(event PolicyEvent) bool {
		if event.RuleResponse != nil && event.RuleResponse.PolicyName == policyName {
			return true
		}
		return false
	}
}

func FilterByOutcome(outcome EventOutcome) EventFilterFunc {
	return func(event PolicyEvent) bool {
		return event.Outcome == outcome
	}
}

func FilterByNamespace(namespace string) EventFilterFunc {
	return func(event PolicyEvent) bool {
		if event.RuleResponse != nil && event.RuleResponse.ResourceNamespace == namespace {
			return true
		}
		return false
	}
}
