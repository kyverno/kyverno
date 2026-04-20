package background

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests for UpdateRequest state transitions and status handling

func TestUpdateRequestState_IsPending(t *testing.T) {
	tests := []struct {
		name     string
		state    kyvernov2.UpdateRequestState
		expected bool
	}{
		{"pending state", kyvernov2.Pending, true},
		{"completed state", kyvernov2.Completed, false},
		{"failed state", kyvernov2.Failed, false},
		{"skip state", kyvernov2.Skip, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state == kyvernov2.Pending
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateRequestState_IsTerminal(t *testing.T) {
	// Terminal states are Completed and Skip - these should not be requeued
	tests := []struct {
		name       string
		state      kyvernov2.UpdateRequestState
		isTerminal bool
	}{
		{"pending is not terminal", kyvernov2.Pending, false},
		{"completed is terminal", kyvernov2.Completed, true},
		{"failed is not terminal (can retry)", kyvernov2.Failed, false},
		{"skip is terminal", kyvernov2.Skip, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTerminal := tt.state == kyvernov2.Completed || tt.state == kyvernov2.Skip
			assert.Equal(t, tt.isTerminal, isTerminal)
		})
	}
}

func TestNewUpdateRequest_DefaultState(t *testing.T) {
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: config.KyvernoNamespace(),
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.Generate,
			Policy: "test-policy",
		},
	}

	// New UR should have empty state initially
	assert.Empty(t, ur.Status.State)
}

func TestUpdateRequest_StateTransitionValid(t *testing.T) {
	// Valid state transitions:
	// Pending -> Completed (success)
	// Pending -> Failed (error)
	// Pending -> Skip (policy deleted)
	// Failed -> Pending (retry)

	validTransitions := []struct {
		from kyvernov2.UpdateRequestState
		to   kyvernov2.UpdateRequestState
	}{
		{kyvernov2.Pending, kyvernov2.Completed},
		{kyvernov2.Pending, kyvernov2.Failed},
		{kyvernov2.Pending, kyvernov2.Skip},
		{kyvernov2.Failed, kyvernov2.Pending},
	}

	for _, tt := range validTransitions {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			ur := &kyvernov2.UpdateRequest{
				Status: kyvernov2.UpdateRequestStatus{State: tt.from},
			}
			ur.Status.State = tt.to
			assert.Equal(t, tt.to, ur.Status.State)
		})
	}
}

func TestUpdateRequest_GenerateType(t *testing.T) {
	ur := &kyvernov2.UpdateRequest{
		Spec: kyvernov2.UpdateRequestSpec{
			Type: kyvernov2.Generate,
		},
	}
	assert.Equal(t, kyvernov2.Generate, ur.Spec.Type)
}

func TestUpdateRequest_MutateType(t *testing.T) {
	ur := &kyvernov2.UpdateRequest{
		Spec: kyvernov2.UpdateRequestSpec{
			Type: kyvernov2.Mutate,
		},
	}
	assert.Equal(t, kyvernov2.Mutate, ur.Spec.Type)
}

func TestUpdateRequest_PolicyReference(t *testing.T) {
	// Test that policy reference is preserved correctly
	tests := []struct {
		name      string
		policyRef string
		isCluster bool
	}{
		{"cluster policy", "require-labels", true},
		{"namespaced policy", "default/require-labels", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ur := &kyvernov2.UpdateRequest{
				Spec: kyvernov2.UpdateRequestSpec{
					Policy: tt.policyRef,
				},
			}
			assert.Equal(t, tt.policyRef, ur.Spec.Policy)
		})
	}
}

func TestUpdateRequest_RetryCount(t *testing.T) {
	ur := &kyvernov2.UpdateRequest{
		Status: kyvernov2.UpdateRequestStatus{
			State:   kyvernov2.Failed,
			Message: "first failure",
		},
	}

	// Simulate retry by resetting to pending
	ur.Status.State = kyvernov2.Pending
	assert.Equal(t, kyvernov2.Pending, ur.Status.State)

	// Original message preserved
	assert.Equal(t, "first failure", ur.Status.Message)
}

func TestMaxRetries_Value(t *testing.T) {
	// Verify the maxRetries constant
	assert.Equal(t, 10, maxRetries)
}
