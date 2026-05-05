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
)

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

func TestHandlerFunc_Execute_ResponseStates(t *testing.T) {
	tests := []struct {
		name    string
		allowed bool
		status  *metav1.Status
	}{
		{
			name:    "allowed without status",
			allowed: true,
			status:  nil,
		},
		{
			name:    "denied with forbidden status",
			allowed: false,
			status: &metav1.Status{
				Message: "denied",
				Reason:  metav1.StatusReasonForbidden,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
				return admissionv1.AdmissionResponse{
					Allowed: tt.allowed,
					Result:  tt.status,
				}
			})

			response := handler.Execute(context.Background(), logr.Discard(), handlers.AdmissionRequest{}, "", time.Now())

			assert.Equal(t, tt.allowed, response.Allowed)
			if tt.status == nil {
				assert.Nil(t, response.Result)
			} else {
				if assert.NotNil(t, response.Result) {
					assert.Equal(t, tt.status.Message, response.Result.Message)
					assert.Equal(t, tt.status.Reason, response.Result.Reason)
				}
			}
		})
	}
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

func TestHandlerFunc_Execute_FailurePolicies(t *testing.T) {
	tests := []struct {
		name          string
		failurePolicy string
	}{
		{
			name:          "with Fail policy",
			failurePolicy: "Fail",
		},
		{
			name:          "with Ignore policy",
			failurePolicy: "Ignore",
		},
		{
			name:          "with empty policy",
			failurePolicy: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPolicy string
			handler := HandlerFunc(func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
				capturedPolicy = failurePolicy
				return admissionv1.AdmissionResponse{Allowed: true}
			})

			response := handler.Execute(context.Background(), logr.Discard(), handlers.AdmissionRequest{}, tt.failurePolicy, time.Now())

			assert.True(t, response.Allowed)
			assert.Equal(t, tt.failurePolicy, capturedPolicy)
		})
	}
}
