package webhooks

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDebugModeOptions_DefaultValues(t *testing.T) {
	opts := DebugModeOptions{}
	
	assert.False(t, opts.DumpPayload, "DumpPayload should be false by default")
}

func TestDebugModeOptions_WithDumpPayload(t *testing.T) {
	opts := DebugModeOptions{
		DumpPayload: true,
	}
	
	assert.True(t, opts.DumpPayload)
}

func TestHandlerFunc_Execute(t *testing.T) {
	expectedResponse := admissionv1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: "test response",
		},
	}
	
	handlerCalled := false
	var capturedCtx context.Context
	var capturedFailurePolicy string
	
	handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		handlerCalled = true
		capturedCtx = ctx
		capturedFailurePolicy = failurePolicy
		return expectedResponse
	})
	
	ctx := context.Background()
	logger := logr.Discard()
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID: "test-uid",
		},
	}
	failurePolicy := "Fail"
	startTime := time.Now()
	
	response := handler.Execute(ctx, logger, request, failurePolicy, startTime)
	
	assert.True(t, handlerCalled, "handler function should be called")
	assert.Equal(t, ctx, capturedCtx)
	assert.Equal(t, failurePolicy, capturedFailurePolicy)
	assert.Equal(t, expectedResponse, response)
}

func TestHandlerFunc_Execute_ReturnsAllowed(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{
			Allowed: true,
		}
	})
	
	response := handler.Execute(context.Background(), logr.Discard(), handlers.AdmissionRequest{}, "", time.Now())
	
	assert.True(t, response.Allowed)
}

func TestHandlerFunc_Execute_ReturnsDenied(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "denied",
				Reason:  metav1.StatusReasonForbidden,
			},
		}
	})
	
	response := handler.Execute(context.Background(), logr.Discard(), handlers.AdmissionRequest{}, "", time.Now())
	
	assert.False(t, response.Allowed)
	assert.Equal(t, "denied", response.Result.Message)
}

func TestHandlerFunc_Execute_WithPatch(t *testing.T) {
	patchType := admissionv1.PatchTypeJSONPatch
	patch := []byte(`[{"op": "add", "path": "/metadata/labels/test", "value": "true"}]`)
	
	handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{
			Allowed:   true,
			PatchType: &patchType,
			Patch:     patch,
		}
	})
	
	response := handler.Execute(context.Background(), logr.Discard(), handlers.AdmissionRequest{}, "", time.Now())
	
	assert.True(t, response.Allowed)
	assert.NotNil(t, response.PatchType)
	assert.Equal(t, admissionv1.PatchTypeJSONPatch, *response.PatchType)
	assert.Equal(t, patch, response.Patch)
}

func TestHandlerFunc_ImplementsHandler(t *testing.T) {
	var handler Handler = HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{Allowed: true}
	})
	
	assert.NotNil(t, handler)
}

func TestHandlerFunc_Execute_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name           string
		failurePolicy  string
		expectedResult bool
	}{
		{
			name:           "with Fail policy",
			failurePolicy:  "Fail",
			expectedResult: true,
		},
		{
			name:           "with Ignore policy",
			failurePolicy:  "Ignore",
			expectedResult: true,
		},
		{
			name:           "with empty policy",
			failurePolicy:  "",
			expectedResult: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPolicy string
			handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
				capturedPolicy = failurePolicy
				return admissionv1.AdmissionResponse{Allowed: tt.expectedResult}
			})
			
			response := handler.Execute(context.Background(), logr.Discard(), handlers.AdmissionRequest{}, tt.failurePolicy, time.Now())
			
			assert.Equal(t, tt.expectedResult, response.Allowed)
			assert.Equal(t, tt.failurePolicy, capturedPolicy)
		})
	}
}

func TestCELExceptionHandlers_Struct(t *testing.T) {
	mockHandler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{Allowed: true}
	})
	
	handlers := CELExceptionHandlers{
		Validation: mockHandler,
	}
	
	assert.NotNil(t, handlers.Validation)
}

func TestExceptionHandlers_Struct(t *testing.T) {
	mockHandler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{Allowed: true}
	})
	
	handlers := ExceptionHandlers{
		Validation: mockHandler,
	}
	
	assert.NotNil(t, handlers.Validation)
}

func TestGlobalContextHandlers_Struct(t *testing.T) {
	mockHandler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{Allowed: true}
	})
	
	handlers := GlobalContextHandlers{
		Validation: mockHandler,
	}
	
	assert.NotNil(t, handlers.Validation)
}

func TestPolicyHandlers_Struct(t *testing.T) {
	mockHandler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{Allowed: true}
	})
	
	handlers := PolicyHandlers{
		Mutation:   mockHandler,
		Validation: mockHandler,
	}
	
	assert.NotNil(t, handlers.Mutation)
	assert.NotNil(t, handlers.Validation)
}

func TestResourceHandlers_Struct(t *testing.T) {
	mockHandler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		return admissionv1.AdmissionResponse{Allowed: true}
	})
	
	handlers := ResourceHandlers{
		Mutation:                          mockHandler,
		Validation:                        mockHandler,
		ValidatingPolicies:                mockHandler,
		NamespacedValidatingPolicies:      mockHandler,
		ImageVerificationPoliciesMutation: mockHandler,
		ImageVerificationPolicies:         mockHandler,
		GeneratingPolicies:                mockHandler,
		NamespacedGeneratingPolicies:      mockHandler,
		MutatingPolicies:                  mockHandler,
		NamespacedMutatingPolicies:        mockHandler,
	}
	
	assert.NotNil(t, handlers.Mutation)
	assert.NotNil(t, handlers.Validation)
	assert.NotNil(t, handlers.ValidatingPolicies)
	assert.NotNil(t, handlers.NamespacedValidatingPolicies)
	assert.NotNil(t, handlers.ImageVerificationPoliciesMutation)
	assert.NotNil(t, handlers.ImageVerificationPolicies)
	assert.NotNil(t, handlers.GeneratingPolicies)
	assert.NotNil(t, handlers.NamespacedGeneratingPolicies)
	assert.NotNil(t, handlers.MutatingPolicies)
	assert.NotNil(t, handlers.NamespacedMutatingPolicies)
}

func TestHandlerFunc_Execute_WithAdmissionRequest(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
		// Verify request is passed correctly
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})
	
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid-123",
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Name:      "test-pod",
			Namespace: "default",
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}
	
	response := handler.Execute(context.Background(), logr.Discard(), request, "Fail", time.Now())
	
	assert.Equal(t, "test-uid-123", string(response.UID))
	assert.True(t, response.Allowed)
}
