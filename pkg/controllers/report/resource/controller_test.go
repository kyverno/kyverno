package resource

import (
	"errors"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/event"
	corev1 "k8s.io/api/core/v1"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// mockEventGenerator is a mock implementation of event.Interface for testing
type mockEventGenerator struct {
	events []event.Info
}

func (m *mockEventGenerator) Add(infos ...event.Info) {
	m.events = append(m.events, infos...)
}

func TestEmitKindResolutionEvent(t *testing.T) {
	tests := []struct {
		name       string
		policyRef  corev1.ObjectReference
		kindStr    string
		err        error
		wantEvents int
	}{
		{
			name: "emit event for ValidatingAdmissionPolicy",
			policyRef: corev1.ObjectReference{
				APIVersion: "admissionregistration.k8s.io/v1",
				Kind:       "ValidatingAdmissionPolicy",
				Name:       "test-policy",
			},
			kindStr:    "apps/v1/Deployment",
			err:        errors.New("no matches for kind \"Deployment\" in version \"apps/v1\""),
			wantEvents: 1,
		},
		{
			name: "emit event for NamespacedValidatingPolicy",
			policyRef: corev1.ObjectReference{
				APIVersion: "policies.kyverno.io/v1beta1",
				Kind:       "NamespacedValidatingPolicy",
				Name:       "test-policy",
				Namespace:  "test-ns",
			},
			kindStr:    "custom.io/v1/CustomResource",
			err:        errors.New("resource not found"),
			wantEvents: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGen := &mockEventGenerator{}
			c := &controller{
				eventGenerator: mockGen,
			}

			c.emitKindResolutionEvent(tt.policyRef, tt.kindStr, tt.err)

			if len(mockGen.events) != tt.wantEvents {
				t.Errorf("expected %d events, got %d", tt.wantEvents, len(mockGen.events))
			}

			if len(mockGen.events) > 0 {
				evt := mockGen.events[0]
				if evt.Reason != event.KindResolutionFailed {
					t.Errorf("expected reason %s, got %s", event.KindResolutionFailed, evt.Reason)
				}
				if evt.Type != corev1.EventTypeWarning {
					t.Errorf("expected type %s, got %s", corev1.EventTypeWarning, evt.Type)
				}
				if evt.Regarding.Name != tt.policyRef.Name {
					t.Errorf("expected regarding name %s, got %s", tt.policyRef.Name, evt.Regarding.Name)
				}
				if evt.Regarding.Kind != tt.policyRef.Kind {
					t.Errorf("expected regarding kind %s, got %s", tt.policyRef.Kind, evt.Regarding.Kind)
				}
				if evt.Regarding.Namespace != tt.policyRef.Namespace {
					t.Errorf("expected regarding namespace %s, got %s", tt.policyRef.Namespace, evt.Regarding.Namespace)
				}
				if evt.Source != event.PolicyController {
					t.Errorf("expected source %s, got %s", event.PolicyController, evt.Source)
				}
				// Verify message contains the failed kind/resource
				if len(evt.Message) == 0 {
					t.Error("expected non-empty message")
				}
				// Message should contain: policy name, kind string, and error
				expectedSubstrings := []string{tt.policyRef.Name, tt.kindStr, tt.err.Error()}
				for _, substr := range expectedSubstrings {
					if len(substr) > 0 && !contains(evt.Message, substr) {
						t.Errorf("expected message to contain %q, got: %s", substr, evt.Message)
					}
				}
			}
		})
	}
}

func TestEmitKindResolutionEvent_NilGenerator(t *testing.T) {
	c := &controller{
		eventGenerator: nil,
	}

	policyRef := corev1.ObjectReference{
		APIVersion: "admissionregistration.k8s.io/v1",
		Kind:       "ValidatingAdmissionPolicy",
		Name:       "test-policy",
	}

	c.emitKindResolutionEvent(policyRef, "apps/v1/Deployment", errors.New("test error"))
}

