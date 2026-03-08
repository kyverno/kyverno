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

func TestEventGenerator_PolicyApplied_SuccessEventsDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientset := fake.NewSimpleClientset()
	eventCreated := make(chan struct{}, 1)
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})

	// generateSuccessEvents=false (default), generateMutationEvents=false (default)
	cfg := config.NewDefaultConfiguration(false)
	eventGenerator := NewEventGenerator(clientset.EventsV1(), logr.Discard(), 1000, cfg)
	go eventGenerator.Run(ctx, Workers)
	time.Sleep(500 * time.Millisecond)

	// PolicyApplied event should be dropped when generateSuccessEvents is false
	eventGenerator.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	select {
	case <-eventCreated:
		t.Fatal("PolicyApplied event should have been dropped")
	case <-time.After(2 * time.Second):
		// expected: event was dropped
	}
}

func TestEventGenerator_PolicyApplied_SuccessEventsEnabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientset := fake.NewSimpleClientset()
	eventCreated := make(chan struct{}, 1)
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{"generateSuccessEvents": "true"},
	})
	eventGenerator := NewEventGenerator(clientset.EventsV1(), logr.Discard(), 1000, cfg)
	go eventGenerator.Run(ctx, Workers)
	time.Sleep(500 * time.Millisecond)

	eventGenerator.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	select {
	case <-eventCreated:
		// expected: event was created
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal("PolicyApplied event should have been created when generateSuccessEvents is true")
	}
}

func TestEventGenerator_MutationEvent_MutationEventsEnabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientset := fake.NewSimpleClientset()
	eventCreated := make(chan struct{}, 1)
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})

	// generateSuccessEvents=false, generateMutationEvents=true
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{"generateMutationEvents": "true"},
	})
	eventGenerator := NewEventGenerator(clientset.EventsV1(), logr.Discard(), 1000, cfg)
	go eventGenerator.Run(ctx, Workers)
	time.Sleep(500 * time.Millisecond)

	eventGenerator.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourceMutated,
		Message:   "resource mutated",
		Source:    AdmissionController,
	})

	select {
	case <-eventCreated:
		// expected: mutation event was created
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal("mutation event should have been created when generateMutationEvents is true")
	}
}

func TestEventGenerator_ValidationEvent_MutationEventsOnly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientset := fake.NewSimpleClientset()
	eventCreated := make(chan struct{}, 1)
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})

	// generateSuccessEvents=false, generateMutationEvents=true
	// validation events should still be dropped
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{"generateMutationEvents": "true"},
	})
	eventGenerator := NewEventGenerator(clientset.EventsV1(), logr.Discard(), 1000, cfg)
	go eventGenerator.Run(ctx, Workers)
	time.Sleep(500 * time.Millisecond)

	eventGenerator.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	select {
	case <-eventCreated:
		t.Fatal("validation event should have been dropped when only generateMutationEvents is true")
	case <-time.After(2 * time.Second):
		// expected: event was dropped
	}
}

func TestEventGenerator_AllSuccessEventsWhenBothFlagsSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientset := fake.NewSimpleClientset()
	eventCreated := make(chan struct{}, 2)
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})

	// both generateSuccessEvents=true and generateMutationEvents=true
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"generateSuccessEvents":  "true",
			"generateMutationEvents": "true",
		},
	})
	eventGenerator := NewEventGenerator(clientset.EventsV1(), logr.Discard(), 1000, cfg)
	go eventGenerator.Run(ctx, Workers)
	time.Sleep(500 * time.Millisecond)

	// mutation event should pass
	eventGenerator.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourceMutated,
		Message:   "resource mutated",
		Source:    AdmissionController,
	})

	select {
	case <-eventCreated:
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal("mutation event should have been created when both flags are true")
	}

	// validation event should also pass
	eventGenerator.Add(Info{
		Regarding: corev1.ObjectReference{Kind: "Pod", Name: "test-pod-2", Namespace: "default"},
		Reason:    PolicyApplied,
		Action:    ResourcePassed,
		Message:   "validation passed",
		Source:    AdmissionController,
	})

	select {
	case <-eventCreated:
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal("validation event should have been created when both flags are true")
	}
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
