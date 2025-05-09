package event

import (
	"errors"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewBackgroundFailedEvent(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		policy         kyvernov1.PolicyInterface
		rule           string
		source         Source
		resource       kyvernov1.ResourceSpec
		expectedMsg    string
		expectedEvents int
	}{
		{
			name: "nil error",
			err:  nil,
			policy: &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
			},
			rule:           "test-rule",
			source:         AdmissionController,
			resource:       kyvernov1.ResourceSpec{Kind: "Pod", Name: "test-pod", Namespace: "default"},
			expectedMsg:    "policy test-policy/test-rule error: no details available",
			expectedEvents: 1,
		},
		{
			name: "object has been modified error",
			err:  fmt.Errorf("object has been modified"),
			policy: &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
			},
			rule:           "test-rule",
			source:         GeneratePolicyController,
			resource:       kyvernov1.ResourceSpec{Kind: "ConfigMap", Name: "test-cm", Namespace: "default"},
			expectedMsg:    "policy test-policy/test-rule error: object has been modified",
			expectedEvents: 1,
		},
		{
			name: "standard error",
			err:  errors.New("standard error"),
			policy: &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
			},
			rule:           "test-rule",
			source:         MutateExistingController,
			resource:       kyvernov1.ResourceSpec{Kind: "Deployment", Name: "test-deploy", Namespace: "default"},
			expectedMsg:    "policy test-policy/test-rule error: standard error",
			expectedEvents: 1,
		},
		{
			name: "no rule specified",
			err:  errors.New("no rule error"),
			policy: &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy-no-rule",
				},
			},
			rule:           "",
			source:         MutateExistingController,
			resource:       kyvernov1.ResourceSpec{Kind: "Service", Name: "test-svc", Namespace: "default"},
			expectedMsg:    "policy test-policy-no-rule error: no rule error",
			expectedEvents: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events := NewBackgroundFailedEvent(tc.err, tc.policy, tc.rule, tc.source, tc.resource)

			if len(events) != tc.expectedEvents {
				t.Errorf("Expected %d events, but got %d", tc.expectedEvents, len(events))
			}

			if events[0].Message != tc.expectedMsg {
				t.Errorf("Expected message '%s', but got '%s'", tc.expectedMsg, events[0].Message)
			}

			// Verify the source and other fields are correctly set
			if events[0].Source != tc.source {
				t.Errorf("Expected source %v, but got %v", tc.source, events[0].Source)
			}

			// Verify the related resource is correctly set
			if events[0].Related.Kind != tc.resource.Kind ||
				events[0].Related.Name != tc.resource.Name ||
				events[0].Related.Namespace != tc.resource.Namespace {
				t.Errorf("Related resource doesn't match expected values")
			}

			// Verify the reason is PolicyError
			if events[0].Reason != PolicyError {
				t.Errorf("Expected reason PolicyError, but got %v", events[0].Reason)
			}
		})
	}
}
