package event

import (
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewBackgroundFailedEvent(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		rule          string
		expectedEvent bool
		expectedMsg   string
	}{
		{
			name:          "nil error should not generate event",
			err:           nil,
			rule:          "test-rule",
			expectedEvent: false,
		},
		{
			name:          "error with rule should generate event with rule in message",
			err:           fmt.Errorf("test error"),
			rule:          "test-rule",
			expectedEvent: true,
			expectedMsg:   "policy test-policy/test-rule error: test error",
		},
		{
			name:          "error without rule should generate event without rule in message",
			err:           fmt.Errorf("test error"),
			rule:          "",
			expectedEvent: true,
			expectedMsg:   "policy test-policy error: test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
			}

			resource := kyvernov1.ResourceSpec{
				Kind:      "Pod",
				Name:      "test-pod",
				Namespace: "default",
			}

			events := NewBackgroundFailedEvent(tt.err, policy, tt.rule, GeneratePolicyController, resource)

			if tt.expectedEvent {
				assert.NotEmpty(t, events, "expected event to be generated")
				if len(events) > 0 {
					assert.Equal(t, tt.expectedMsg, events[0].Message)
					assert.Equal(t, PolicyError, events[0].Reason)
					assert.Equal(t, GeneratePolicyController, events[0].Source)
					assert.Equal(t, None, events[0].Action)
				}
			} else {
				assert.Empty(t, events, "expected no events to be generated")
			}
		})
	}
}
