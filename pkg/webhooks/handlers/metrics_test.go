package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAdmissionHandler_withMetrics_NilMetrics(t *testing.T) {
	handlerCalled := false
	expectedResponse := admissionv1.AdmissionResponse{
		Allowed: true,
		UID:     "test-uid",
	}

	innerHandler := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		handlerCalled = true
		return expectedResponse
	}

	wrappedHandler := AdmissionHandler(innerHandler).withMetrics()

	ctx := context.Background()
	logger := logr.Discard()
	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: "test-namespace",
			Operation: admissionv1.Create,
			Kind: metav1.GroupVersionKind{
				Kind: "Pod",
			},
		},
	}
	startTime := time.Now()

	response := wrappedHandler(ctx, logger, request, startTime)

	assert.True(t, handlerCalled, "Inner handler should be called")
	assert.True(t, response.Allowed, "Response should be allowed")
	assert.Equal(t, expectedResponse.UID, response.UID)
}

func TestAdmissionHandler_withMetrics_PassesThroughResponse(t *testing.T) {
	tests := []struct {
		name     string
		response admissionv1.AdmissionResponse
	}{
		{
			name: "allowed response",
			response: admissionv1.AdmissionResponse{
				Allowed: true,
				UID:     "test-uid-1",
			},
		},
		{
			name: "denied response",
			response: admissionv1.AdmissionResponse{
				Allowed: false,
				UID:     "test-uid-2",
				Result: &metav1.Status{
					Message: "Request denied",
					Code:    403,
				},
			},
		},
		{
			name: "response with patch",
			response: admissionv1.AdmissionResponse{
				Allowed: true,
				UID:     "test-uid-3",
				Patch:   []byte(`[{"op":"add","path":"/metadata/labels/test","value":"value"}]`),
				PatchType: func() *admissionv1.PatchType {
					pt := admissionv1.PatchTypeJSONPatch
					return &pt
				}(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerHandler := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				return tt.response
			}

			wrappedHandler := AdmissionHandler(innerHandler).withMetrics()

			ctx := context.Background()
			logger := logr.Discard()
			request := AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Namespace: "test-namespace",
					Operation: admissionv1.Create,
					Kind: metav1.GroupVersionKind{
						Kind: "Pod",
					},
				},
			}
			startTime := time.Now()

			response := wrappedHandler(ctx, logger, request, startTime)

			assert.Equal(t, tt.response.Allowed, response.Allowed)
			assert.Equal(t, tt.response.UID, response.UID)
			if tt.response.Result != nil {
				assert.Equal(t, tt.response.Result.Message, response.Result.Message)
				assert.Equal(t, tt.response.Result.Code, response.Result.Code)
			}
			if tt.response.Patch != nil {
				assert.Equal(t, tt.response.Patch, response.Patch)
				assert.Equal(t, tt.response.PatchType, response.PatchType)
			}
		})
	}
}

func TestAdmissionHandler_withMetrics_DifferentOperations(t *testing.T) {
	operations := []admissionv1.Operation{
		admissionv1.Create,
		admissionv1.Update,
		admissionv1.Delete,
		admissionv1.Connect,
	}

	for _, op := range operations {
		t.Run(string(op), func(t *testing.T) {
			handlerCalled := false
			innerHandler := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				handlerCalled = true
				assert.Equal(t, op, request.Operation)
				return admissionv1.AdmissionResponse{
					Allowed: true,
				}
			}

			wrappedHandler := AdmissionHandler(innerHandler).withMetrics()

			ctx := context.Background()
			logger := logr.Discard()
			request := AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Namespace: "test-namespace",
					Operation: op,
					Kind: metav1.GroupVersionKind{
						Kind: "Deployment",
					},
				},
			}
			startTime := time.Now()

			response := wrappedHandler(ctx, logger, request, startTime)

			assert.True(t, handlerCalled, "Inner handler should be called")
			assert.True(t, response.Allowed)
		})
	}
}

func TestAdmissionHandler_withMetrics_WithAttributes(t *testing.T) {
	handlerCalled := false
	innerHandler := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		handlerCalled = true
		return admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	attrs := []attribute.KeyValue{
		attribute.String("handler", "test-handler"),
		attribute.Int("priority", 1),
		attribute.Bool("background", false),
	}

	wrappedHandler := AdmissionHandler(innerHandler).withMetrics(attrs...)

	ctx := context.Background()
	logger := logr.Discard()
	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: "test-namespace",
			Operation: admissionv1.Create,
			Kind: metav1.GroupVersionKind{
				Kind: "Pod",
			},
		},
	}
	startTime := time.Now()

	response := wrappedHandler(ctx, logger, request, startTime)

	assert.True(t, handlerCalled, "Inner handler should be called")
	assert.True(t, response.Allowed)
}

func TestHttpHandler_withMetrics_NilMetrics(t *testing.T) {
	handlerCalled := false
	innerHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}

	wrappedHandler := HttpHandler(innerHandler).withMetrics()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler(w, req)

	assert.True(t, handlerCalled, "Inner handler should be called")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestHttpHandler_withMetrics_DifferentMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
		uri    string
	}{
		{
			name:   "GET request",
			method: http.MethodGet,
			uri:    "/api/resource",
		},
		{
			name:   "POST request",
			method: http.MethodPost,
			uri:    "/api/create",
		},
		{
			name:   "DELETE request",
			method: http.MethodDelete,
			uri:    "/api/delete/123",
		},
		{
			name:   "PUT request",
			method: http.MethodPut,
			uri:    "/api/update/456",
		},
		{
			name:   "PATCH request",
			method: http.MethodPatch,
			uri:    "/api/patch/789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			var capturedMethod string
			var capturedURI string

			innerHandler := func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				capturedMethod = r.Method
				capturedURI = r.RequestURI
				w.WriteHeader(http.StatusOK)
			}

			wrappedHandler := HttpHandler(innerHandler).withMetrics()

			req := httptest.NewRequest(tt.method, tt.uri, nil)
			w := httptest.NewRecorder()

			wrappedHandler(w, req)

			assert.True(t, handlerCalled, "Inner handler should be called")
			assert.Equal(t, tt.method, capturedMethod, "Method should match")
			assert.Equal(t, tt.uri, capturedURI, "URI should match")
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestHttpHandler_withMetrics_DifferentResponseCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "200 OK",
			statusCode: http.StatusOK,
			body:       "success",
		},
		{
			name:       "201 Created",
			statusCode: http.StatusCreated,
			body:       "created",
		},
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			body:       "bad request",
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			body:       "not found",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			body:       "server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerHandler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}

			wrappedHandler := HttpHandler(innerHandler).withMetrics()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			wrappedHandler(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, tt.body, w.Body.String())
		})
	}
}

func TestHttpHandler_withMetrics_WithAttributes(t *testing.T) {
	handlerCalled := false
	innerHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	attrs := []attribute.KeyValue{
		attribute.String("handler", "test-handler"),
		attribute.String("version", "v1"),
	}

	wrappedHandler := HttpHandler(innerHandler).withMetrics(attrs...)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler(w, req)

	assert.True(t, handlerCalled, "Inner handler should be called")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpHandler_withMetrics_ExecutionTiming(t *testing.T) {
	executionTime := 50 * time.Millisecond

	innerHandler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(executionTime)
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := HttpHandler(innerHandler).withMetrics()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	startTime := time.Now()
	wrappedHandler(w, req)
	duration := time.Since(startTime)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, duration, executionTime, "Handler should have taken at least the execution time")
}
