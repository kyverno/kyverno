package event

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/event"
	corev1 "k8s.io/api/core/v1"
)

func TestNewErrorEvent(t *testing.T) {
	regarding := corev1.ObjectReference{
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "test-pod",
		UID:        "12345",
		APIVersion: "v1",
	}

	err := fmt.Errorf("some error")

	generated_event := NewErrorEvent(regarding, err)

	if generated_event.Regarding != regarding {
		t.Errorf("Expected Regarding to be %v, but got %v", regarding, generated_event.Regarding)
	}

	if generated_event.Source != source {
		t.Errorf("Expected Source to be %s, but got %s", source, generated_event.Source)
	}

	if generated_event.Reason != event.PolicyError {
		t.Errorf("Expected Reason to be %s, but got %s", event.PolicyError, generated_event.Reason)
	}

	if generated_event.Message != err.Error() {
		t.Errorf("Expected Message to be %s, but got %s", err.Error(), generated_event.Message)
	}

	if generated_event.Action != action {
		t.Errorf("Expected Action to be %s, but got %s", action, generated_event.Action)
	}

	if generated_event.Type != corev1.EventTypeWarning {
		t.Errorf("Expected Type to be %s, but got %s", corev1.EventTypeWarning, generated_event.Type)
	}
}
