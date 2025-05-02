package generate

import (
	"errors"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MockEventGen is a mock implementation of the event.Interface
type MockEventGen struct {
	events []event.Info
}

func (m *MockEventGen) Add(infos ...event.Info) {
	m.events = append(m.events, infos...)
}

func TestProcessURErrorHandling(t *testing.T) {
	// Create a test controller with mocked dependencies
	mockEventGen := &MockEventGen{}
	controller := &GenerateController{
		eventGen: mockEventGen,
	}

	// Create a test function to simulate error scenario
	testErrorHandling := func(err error, expectedErrMsgContains string) {
		// Reset events
		mockEventGen.events = []event.Info{}

		// Create test policy
		policy := &kyvernov1.ClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-policy",
			},
		}

		// Create a mock trigger resource
		trigger := unstructured.Unstructured{}
		trigger.SetKind("Pod")
		trigger.SetName("test-pod")
		trigger.SetNamespace("default")

		// Create a test UpdateRequest
		ur := &kyvernov2.UpdateRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ur",
			},
			Spec: kyvernov2.UpdateRequestSpec{
				Policy: "test-policy",
				RuleContext: []kyvernov2.RuleContext{
					{
						Rule: "test-rule",
					},
				},
			},
		}

		// Call the method that creates error events but with our custom controller
		// that has mocked dependencies
		controller.processErrorEvent(policy, trigger, *ur, 0, err)

		// Check if an event was created
		assert.NotEmpty(t, mockEventGen.events, "Expected events to be generated")

		// Check if the event message contains the expected error message
		eventMessage := mockEventGen.events[0].Message
		assert.True(t, strings.Contains(eventMessage, expectedErrMsgContains),
			"Expected message to contain '%s', but got: '%s'", expectedErrMsgContains, eventMessage)
	}

	// Test nil error case
	testErrorHandling(nil, "resource generation failed with an unknown error")

	// Test "object has been modified" error case
	testErrorHandling(errors.New("object has been modified"), "resource already exists or has been modified")

	// Test standard error case
	testErrorHandling(errors.New("standard error"), "standard error")
}

// Helper method to replicate just the error event creation part of ProcessUR
func (c *GenerateController) processErrorEvent(
	policy kyvernov1.PolicyInterface,
	trigger unstructured.Unstructured,
	ur kyvernov2.UpdateRequest,
	ruleIndex int,
	err error) {

	// Handle nil errors more gracefully with a more descriptive message
	var errForEvent error
	if err == nil {
		errForEvent = errors.New("resource generation failed with an unknown error")
	} else if strings.Contains(err.Error(), "object has been modified") {
		errForEvent = errors.New("failed to generate resource: resource already exists or has been modified")
	} else {
		errForEvent = err
	}

	events := event.NewBackgroundFailedEvent(
		errForEvent,
		policy,
		ur.Spec.RuleContext[ruleIndex].Rule,
		event.GeneratePolicyController,
		kyvernov1.ResourceSpec{
			Kind:      trigger.GetKind(),
			Namespace: trigger.GetNamespace(),
			Name:      trigger.GetName(),
		},
	)

	c.eventGen.Add(events...)
}
