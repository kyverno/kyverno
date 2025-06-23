package event

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
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
	eventGenerator := NewEventGenerator(eventsClient, logger, 1000)

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
