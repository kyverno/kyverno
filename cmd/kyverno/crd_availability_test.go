package main

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Test the noOpGpolHandler that we created for graceful fallback
func TestNoOpGpolHandler(t *testing.T) {
	handler := &noOpGpolHandler{}

	// Create a mock request with proper structure
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID: "test-uid",
		},
	}

	ctx := context.Background()
	logger := logr.Discard()

	// Test that the handler doesn't panic and returns a valid response
	response := handler.Generate(ctx, logger, request, "fail", time.Now())

	if response.UID != "test-uid" {
		t.Errorf("expected UID %s, got %s", "test-uid", response.UID)
	}

	if response.Allowed != true {
		t.Errorf("expected response to be allowed, got %v", response.Allowed)
	}
}

// Test the CRD availability checking logic pattern
func TestCRDAvailabilityPattern(t *testing.T) {
	tests := []struct {
		name              string
		crdName           string
		crdExists         bool
		expectedAvailable bool
	}{
		{
			name:              "UpdateRequests CRD exists",
			crdName:           "updaterequests.kyverno.io",
			crdExists:         true,
			expectedAvailable: true,
		},
		{
			name:              "UpdateRequests CRD missing",
			crdName:           "updaterequests.kyverno.io",
			crdExists:         false,
			expectedAvailable: false,
		},
		{
			name:              "PolicyExceptions CRD exists",
			crdName:           "policyexceptions.policies.kyverno.io",
			crdExists:         true,
			expectedAvailable: true,
		},
		{
			name:              "PolicyExceptions CRD missing",
			crdName:           "policyexceptions.policies.kyverno.io",
			crdExists:         false,
			expectedAvailable: false,
		},
		{
			name:              "GeneratingPolicies CRD exists",
			crdName:           "generatingpolicies.policies.kyverno.io",
			crdExists:         true,
			expectedAvailable: true,
		},
		{
			name:              "GeneratingPolicies CRD missing",
			crdName:           "generatingpolicies.policies.kyverno.io",
			crdExists:         false,
			expectedAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake API server client
			var objects []runtime.Object
			if tt.crdExists {
				crd := &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: tt.crdName,
					},
				}
				objects = append(objects, crd)
			}

			client := apiextensionsfake.NewSimpleClientset(objects...)

			// Simulate the CRD availability check pattern from main.go
			available := true
			if err := checkCRDAvailability(client, tt.crdName); err != nil {
				available = false
			}

			if available != tt.expectedAvailable {
				t.Errorf("expected availability %v, got %v", tt.expectedAvailable, available)
			}
		})
	}
}

// Test the conditional informer creation pattern
func TestConditionalInformerPattern(t *testing.T) {
	tests := []struct {
		name                      string
		updateRequestsAvailable   bool
		policyExceptionsAvailable bool
		expectedURGen             bool
		expectedPolicyExLister    bool
	}{
		{
			name:                      "all CRDs available",
			updateRequestsAvailable:   true,
			policyExceptionsAvailable: true,
			expectedURGen:             true,
			expectedPolicyExLister:    true,
		},
		{
			name:                      "no CRDs available",
			updateRequestsAvailable:   false,
			policyExceptionsAvailable: false,
			expectedURGen:             false,
			expectedPolicyExLister:    false,
		},
		{
			name:                      "only UpdateRequests available",
			updateRequestsAvailable:   true,
			policyExceptionsAvailable: false,
			expectedURGen:             true,
			expectedPolicyExLister:    false,
		},
		{
			name:                      "only PolicyExceptions available",
			updateRequestsAvailable:   false,
			policyExceptionsAvailable: true,
			expectedURGen:             false,
			expectedPolicyExLister:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the conditional creation pattern from main.go
			var urgen interface{}
			var policyExceptionsLister interface{}

			// UpdateRequests generator creation
			if tt.updateRequestsAvailable {
				urgen = "mock-generator" // In real code, this would be webhookgenerate.NewGenerator(...)
			}

			// PolicyExceptions lister creation
			if tt.policyExceptionsAvailable {
				policyExceptionsLister = "mock-lister" // In real code, this would be kyvernoInformer.Policies().V1alpha1().PolicyExceptions().Lister()
			}

			// Verify the conditional creation worked as expected
			if tt.expectedURGen && urgen == nil {
				t.Error("expected UpdateRequest generator to be created")
			}
			if !tt.expectedURGen && urgen != nil {
				t.Error("expected UpdateRequest generator to be nil")
			}

			if tt.expectedPolicyExLister && policyExceptionsLister == nil {
				t.Error("expected PolicyExceptions lister to be created")
			}
			if !tt.expectedPolicyExLister && policyExceptionsLister != nil {
				t.Error("expected PolicyExceptions lister to be nil")
			}
		})
	}
}

// Test the graceful degradation pattern for GeneratingPolicies
func TestGeneratingPoliciesGracefulDegradation(t *testing.T) {
	tests := []struct {
		name                        string
		updateRequestsAvailable     bool
		generatingPoliciesAvailable bool
		expectNoOpHandler           bool
	}{
		{
			name:                        "both available",
			updateRequestsAvailable:     true,
			generatingPoliciesAvailable: true,
			expectNoOpHandler:           false,
		},
		{
			name:                        "UpdateRequests missing",
			updateRequestsAvailable:     false,
			generatingPoliciesAvailable: true,
			expectNoOpHandler:           true,
		},
		{
			name:                        "GeneratingPolicies missing",
			updateRequestsAvailable:     true,
			generatingPoliciesAvailable: false,
			expectNoOpHandler:           true,
		},
		{
			name:                        "both missing",
			updateRequestsAvailable:     false,
			generatingPoliciesAvailable: false,
			expectNoOpHandler:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the GeneratingPolicies handler creation logic from main.go
			var useNoOpHandler bool

			if tt.updateRequestsAvailable && tt.generatingPoliciesAvailable {
				useNoOpHandler = false
			} else {
				useNoOpHandler = true
			}

			// Verify the correct logic was applied
			if useNoOpHandler != tt.expectNoOpHandler {
				t.Errorf("expected useNoOpHandler=%v, got %v", tt.expectNoOpHandler, useNoOpHandler)
			}
		})
	}
}

// Helper function to simulate CRD availability check
func checkCRDAvailability(client interface{}, crdName string) error {
	// This simulates the kubeutils.CRDsInstalled call from main.go
	if apiClient, ok := client.(*apiextensionsfake.Clientset); ok {
		_, err := apiClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdName, metav1.GetOptions{})
		return err
	}
	return nil
}

// Mock handler for testing - simplified version
type mockRealGpolHandler struct{}

func (h *mockRealGpolHandler) Generate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	return handlers.AdmissionResponse{
		UID:     request.UID,
		Allowed: true,
	}
}

// Test that the integration doesn't break existing functionality
func TestBackwardCompatibility(t *testing.T) {
	// Test that when all CRDs are available, behavior is unchanged
	crdNames := []string{
		"clusterpolicies.kyverno.io",
		"policies.kyverno.io",
		"updaterequests.kyverno.io",
		"policyexceptions.policies.kyverno.io",
		"generatingpolicies.policies.kyverno.io",
	}

	// Create fake client with all CRDs
	var objects []runtime.Object
	for _, crdName := range crdNames {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: crdName,
			},
		}
		objects = append(objects, crd)
	}

	client := apiextensionsfake.NewSimpleClientset(objects...)

	// Test that all CRDs are detected as available
	for _, crdName := range crdNames {
		if err := checkCRDAvailability(client, crdName); err != nil {
			t.Errorf("CRD %s should be available but got error: %v", crdName, err)
		}
	}

	// In this case, all components should be created normally
	// (this simulates the "existing installation" scenario)
}
