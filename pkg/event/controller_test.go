package event

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
)

func TestEventGenerator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCreated := make(chan struct{})
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})

	logger := logr.Discard()

	eventsClient := clientset.EventsV1()
	eventGenerator := NewEventGenerator(eventsClient, logger, 1000, config.NewDefaultConfiguration(false))

	go eventGenerator.Run(ctx, Workers)
	time.Sleep(1 * time.Second)

	info := Info{
		Regarding: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "pod",
			Namespace: "default",
		},
		Reason:  "TestReason",
		Action:  "TestAction",
		Message: "TestMessage",
		Source:  PolicyController,
	}

	eventGenerator.Add(info)

	select {
	case <-eventCreated:
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal("event not created")
	}
}

// newTestEventGenerator creates an event generator for testing and starts its workers.
// The returned channel receives a signal each time an event is created.
func newTestEventGenerator(ctx context.Context, cfg config.Configuration) (*controller, chan struct{}) {
	clientset := fake.NewSimpleClientset()
	eventCreated := make(chan struct{}, 10)
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})
	gen := NewEventGenerator(clientset.EventsV1(), logr.Discard(), 1000, cfg)
	go gen.Run(ctx, Workers)
	return gen, eventCreated
}

// expectEvent waits for an event to be created or fails the test.
func expectEvent(t *testing.T, ch chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal(msg)
	}
}

// expectNoEvent asserts that no event is created within a short window.
// Dropped events never reach the queue, so a brief timeout is sufficient.
func expectNoEvent(t *testing.T, ch chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEventGenerator_PolicyApplied_SuccessEventsDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.NewDefaultConfiguration(false)
	gen, eventCreated := newTestEventGenerator(ctx, cfg)

	gen.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	expectNoEvent(t, eventCreated, "PolicyApplied event should have been dropped")
}

func TestEventGenerator_PolicyApplied_SuccessEventsEnabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{"generateSuccessEvents": "true"},
	})
	gen, eventCreated := newTestEventGenerator(ctx, cfg)

	gen.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	expectEvent(t, eventCreated, "PolicyApplied event should have been created when generateSuccessEvents is true")
}

func TestEventGenerator_SuccessEventActions_FiltersByAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"generateSuccessEvents": "true",
			"successEventActions":   "Resource Mutated",
		},
	})
	gen, eventCreated := newTestEventGenerator(ctx, cfg)

	gen.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourceMutated,
		Message:   "resource mutated",
		Source:    AdmissionController,
	})

	expectEvent(t, eventCreated, "mutation event should have been created with successEventActions=Resource Mutated")
}

func TestEventGenerator_SuccessEventActions_DropsNonMatchingAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"generateSuccessEvents": "true",
			"successEventActions":   "Resource Mutated",
		},
	})
	gen, eventCreated := newTestEventGenerator(ctx, cfg)

	gen.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	expectNoEvent(t, eventCreated, "validation event should have been dropped when successEventActions only includes Resource Mutated")
}

func TestEventGenerator_SuccessEventActions_SuccessEventsDisabledOverrides(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"generateSuccessEvents": "false",
			"successEventActions":   "Resource Mutated",
		},
	})
	gen, eventCreated := newTestEventGenerator(ctx, cfg)

	gen.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourceMutated,
		Message:   "resource mutated",
		Source:    AdmissionController,
	})

	expectNoEvent(t, eventCreated, "event should have been dropped when generateSuccessEvents is false")
}

func TestEventGenerator_SuccessEventActions_MultipleActions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"generateSuccessEvents": "true",
			"successEventActions":   "Resource Mutated,Resource Passed",
		},
	})
	gen, eventCreated := newTestEventGenerator(ctx, cfg)

	gen.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourceMutated,
		Message:   "resource mutated",
		Source:    AdmissionController,
	})

	expectEvent(t, eventCreated, "mutation event should pass with multiple successEventActions")

	gen.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod-2", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	expectEvent(t, eventCreated, "validation event should pass with multiple successEventActions")
}

// TestEventNameSanitization tests that events with invalid RFC 1123 characters in their names are sanitized correctly
func TestEventNameSanitization(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Colon in name",
			input:    "kyverno:migrate-resources",
			expected: "kyverno-migrate-resources",
		},
		{
			name:     "Uppercase characters",
			input:    "KyvernoMigrateResources",
			expected: "kyvernomigrateresources",
		},
		{
			name:     "Multiple invalid characters",
			input:    "kyverno:migrate/resources@1.0",
			expected: "kyverno-migrate-resources-1.0",
		},
		{
			name:     "Name starting with non-alphanumeric",
			input:    "-kyverno-migrate",
			expected: "a-kyverno-migrate",
		},
		{
			name:     "Name ending with non-alphanumeric",
			input:    "kyverno-migrate-",
			expected: "kyverno-migrate-z",
		},
		{
			name:     "Multiple periods",
			input:    "kyverno.migrate.resources",
			expected: "kyverno.migrate.resources",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeEventName(tc.input)
			if result != tc.expected {
				t.Errorf("Expected '%s' to be sanitized to '%s', but got '%s'",
					tc.input, tc.expected, result)
			}
		})
	}

	// Also test the actual event name generation
	mockController := &controller{
		logger: logging.WithName("mock-controller"),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-queue"},
		),
		clock:    clock.RealClock{},
		hostname: "test-host",
	}

	// Create an event with a name that contains invalid characters
	objectWithInvalidChars := corev1.ObjectReference{
		Kind:      "ClusterRole",
		Name:      "kyverno:migrate-resources",
		Namespace: "default",
	}

	// Create the event info
	eventInfo := Info{
		Regarding: objectWithInvalidChars,
		Reason:    PolicyApplied,
		Message:   "Test message",
		Action:    "Test action",
		Source:    Source("test-source"),
		Type:      corev1.EventTypeNormal,
	}

	// Call emitEvent
	mockController.emitEvent(eventInfo)

	// Get the event from the queue
	queueItem, _ := mockController.queue.Get()
	event, ok := queueItem.(*eventsv1.Event)
	if !ok {
		t.Fatalf("Expected an event in the queue, got something else: %v", queueItem)
	}

	// Check that no colons are in the event name
	if event != nil && strings.Contains(event.Name, ":") {
		t.Errorf("Event name still contains colons: %s", event.Name)
	}

	// Verify the name starts with the sanitized resource name
	sanitizedResourceName := "kyverno-migrate-resources"
	if event != nil && !strings.HasPrefix(event.Name, sanitizedResourceName) {
		t.Errorf("Expected name to start with '%s', got: %s",
			sanitizedResourceName, event.Name)
	}
}
