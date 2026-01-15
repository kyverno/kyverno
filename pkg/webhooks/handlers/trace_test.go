package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestHttpHandler_WithTrace(t *testing.T) {
	tests := []struct {
		name           string
		handlerName    string
		method         string
		path           string
		contentLength  int64
		host           string
		expectCalled   bool
		responseStatus int
		responseBody   string
	}{
		{
			name:           "GET request with trace",
			handlerName:    "TestHandler",
			method:         http.MethodGet,
			path:           "/test/path",
			contentLength:  0,
			host:           "example.com",
			expectCalled:   true,
			responseStatus: http.StatusOK,
			responseBody:   "success",
		},
		{
			name:           "POST request with trace",
			handlerName:    "PostHandler",
			method:         http.MethodPost,
			path:           "/api/resource",
			contentLength:  123,
			host:           "api.example.com",
			expectCalled:   true,
			responseStatus: http.StatusCreated,
			responseBody:   "created",
		},
		{
			name:           "PUT request with trace",
			handlerName:    "UpdateHandler",
			method:         http.MethodPut,
			path:           "/api/resource/123",
			contentLength:  456,
			host:           "api.example.com",
			expectCalled:   true,
			responseStatus: http.StatusOK,
			responseBody:   "updated",
		},
		{
			name:           "DELETE request with trace",
			handlerName:    "DeleteHandler",
			method:         http.MethodDelete,
			path:           "/api/resource/456",
			contentLength:  0,
			host:           "api.example.com",
			expectCalled:   true,
			responseStatus: http.StatusNoContent,
			responseBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			innerHandler := HttpHandler(func(writer http.ResponseWriter, request *http.Request) {
				called = true
				assert.Equal(t, tt.method, request.Method)
				assert.Equal(t, tt.path, request.URL.Path)
				writer.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					_, _ = writer.Write([]byte(tt.responseBody))
				}
			})

			tracedHandler := innerHandler.WithTrace(tt.handlerName)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Host = tt.host
			req.ContentLength = tt.contentLength
			rr := httptest.NewRecorder()

			tracedHandler(rr, req)

			assert.Equal(t, tt.expectCalled, called, "inner handler should be called")
			assert.Equal(t, tt.responseStatus, rr.Code, "response status should match")
			if tt.responseBody != "" {
				assert.Equal(t, tt.responseBody, rr.Body.String(), "response body should match")
			}
		})
	}
}

func TestAdmissionHandler_WithTrace(t *testing.T) {
	logger := testr.New(t)

	tests := []struct {
		name             string
		handlerName      string
		request          AdmissionRequest
		innerResponse    AdmissionResponse
		expectCalled     bool
		validateResponse func(t *testing.T, response AdmissionResponse)
	}{
		{
			name:        "admission request with allowed response",
			handlerName: "ValidateHandler",
			request: AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("test-uid-123"),
					Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
					Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Name:      "test-pod",
					Namespace: "default",
					Operation: admissionv1.Create,
					UserInfo: authenticationv1.UserInfo{
						Username: "test-user",
						UID:      "user-123",
						Groups:   []string{"system:authenticated", "developers"},
					},
					SubResource: "",
					RequestKind: &metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Pod",
					},
					RequestResource: &metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
					RequestSubResource: "",
					DryRun:             func() *bool { b := false; return &b }(),
				},
				Roles:            []string{"developer"},
				ClusterRoles:     []string{"view"},
				GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			},
			innerResponse: AdmissionResponse{
				UID:     types.UID("test-uid-123"),
				Allowed: true,
				Result: &metav1.Status{
					Status:  "Success",
					Message: "Pod is allowed",
					Code:    http.StatusOK,
				},
				Warnings: []string{"Warning: deprecated API"},
			},
			expectCalled: true,
			validateResponse: func(t *testing.T, response AdmissionResponse) {
				assert.Equal(t, types.UID("test-uid-123"), response.UID)
				assert.True(t, response.Allowed)
				assert.Equal(t, "Pod is allowed", response.Result.Message)
				assert.Equal(t, []string{"Warning: deprecated API"}, response.Warnings)
			},
		},
		{
			name:        "admission request with denied response",
			handlerName: "DenyHandler",
			request: AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("test-uid-456"),
					Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
					Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
					Name:      "test-deployment",
					Namespace: "production",
					Operation: admissionv1.Update,
					UserInfo: authenticationv1.UserInfo{
						Username: "admin-user",
						UID:      "admin-456",
						Groups:   []string{"system:masters"},
					},
					RequestKind: &metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					RequestResource: &metav1.GroupVersionResource{
						Group:    "apps",
						Version:  "v1",
						Resource: "deployments",
					},
				},
				Roles:            []string{"admin"},
				ClusterRoles:     []string{"cluster-admin"},
				GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
			innerResponse: AdmissionResponse{
				UID:     types.UID("test-uid-456"),
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: "Deployment violates security policy",
					Reason:  metav1.StatusReasonForbidden,
					Code:    http.StatusForbidden,
				},
			},
			expectCalled: true,
			validateResponse: func(t *testing.T, response AdmissionResponse) {
				assert.Equal(t, types.UID("test-uid-456"), response.UID)
				assert.False(t, response.Allowed)
				assert.Equal(t, "Deployment violates security policy", response.Result.Message)
				assert.Equal(t, metav1.StatusReasonForbidden, response.Result.Reason)
			},
		},
		{
			name:        "admission request with patch",
			handlerName: "MutateHandler",
			request: AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("test-uid-789"),
					Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
					Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
					Name:      "test-config",
					Namespace: "default",
					Operation: admissionv1.Create,
					UserInfo: authenticationv1.UserInfo{
						Username: "service-account",
						UID:      "sa-789",
					},
					RequestKind: &metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "ConfigMap",
					},
					RequestResource: &metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "configmaps",
					},
				},
				GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			},
			innerResponse: AdmissionResponse{
				UID:     types.UID("test-uid-789"),
				Allowed: true,
				PatchType: func() *admissionv1.PatchType {
					pt := admissionv1.PatchTypeJSONPatch
					return &pt
				}(),
				Patch: []byte(`[{"op":"add","path":"/metadata/labels/injected","value":"true"}]`),
				Result: &metav1.Status{
					Status:  "Success",
					Message: "ConfigMap mutated",
					Code:    http.StatusOK,
				},
			},
			expectCalled: true,
			validateResponse: func(t *testing.T, response AdmissionResponse) {
				assert.Equal(t, types.UID("test-uid-789"), response.UID)
				assert.True(t, response.Allowed)
				assert.NotNil(t, response.PatchType)
				assert.Equal(t, admissionv1.PatchTypeJSONPatch, *response.PatchType)
				assert.NotEmpty(t, response.Patch)
			},
		},
		{
			name:        "admission request with subresource",
			handlerName: "SubResourceHandler",
			request: AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:         types.UID("test-uid-sub"),
					Kind:        metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
					Resource:    metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					SubResource: "status",
					Name:        "test-pod",
					Namespace:   "default",
					Operation:   admissionv1.Update,
					UserInfo: authenticationv1.UserInfo{
						Username: "controller",
					},
					RequestKind: &metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Pod",
					},
					RequestResource: &metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
					RequestSubResource: "status",
				},
				GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			},
			innerResponse: AdmissionResponse{
				UID:     types.UID("test-uid-sub"),
				Allowed: true,
				Result: &metav1.Status{
					Status: "Success",
					Code:   http.StatusOK,
				},
			},
			expectCalled: true,
			validateResponse: func(t *testing.T, response AdmissionResponse) {
				assert.Equal(t, types.UID("test-uid-sub"), response.UID)
				assert.True(t, response.Allowed)
			},
		},
		{
			name:        "admission request for delete operation",
			handlerName: "DeleteHandler",
			request: AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("test-uid-delete"),
					Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
					Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
					Name:      "test-statefulset",
					Namespace: "production",
					Operation: admissionv1.Delete,
					UserInfo: authenticationv1.UserInfo{
						Username: "admin",
						Groups:   []string{"system:masters"},
					},
					RequestKind: &metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "StatefulSet",
					},
					RequestResource: &metav1.GroupVersionResource{
						Group:    "apps",
						Version:  "v1",
						Resource: "statefulsets",
					},
				},
				GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
			},
			innerResponse: AdmissionResponse{
				UID:     types.UID("test-uid-delete"),
				Allowed: true,
				Result: &metav1.Status{
					Status: "Success",
					Code:   http.StatusOK,
				},
			},
			expectCalled: true,
			validateResponse: func(t *testing.T, response AdmissionResponse) {
				assert.Equal(t, types.UID("test-uid-delete"), response.UID)
				assert.True(t, response.Allowed)
			},
		},
		{
			name:        "admission request with response but no result",
			handlerName: "NoResultHandler",
			request: AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("test-uid-noresult"),
					Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
					Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
					Name:      "test-secret",
					Namespace: "default",
					Operation: admissionv1.Create,
					UserInfo: authenticationv1.UserInfo{
						Username: "user",
					},
					RequestKind: &metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Secret",
					},
					RequestResource: &metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "secrets",
					},
				},
				GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			},
			innerResponse: AdmissionResponse{
				UID:     types.UID("test-uid-noresult"),
				Allowed: true,
				Result:  nil,
			},
			expectCalled: true,
			validateResponse: func(t *testing.T, response AdmissionResponse) {
				assert.Equal(t, types.UID("test-uid-noresult"), response.UID)
				assert.True(t, response.Allowed)
				assert.Nil(t, response.Result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			startTime := time.Now()

			innerHandler := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, requestStartTime time.Time) AdmissionResponse {
				called = true
				assert.Equal(t, tt.request.UID, request.UID)
				assert.Equal(t, tt.request.Operation, request.Operation)
				assert.Equal(t, tt.request.Kind, request.Kind)
				assert.Equal(t, tt.request.Name, request.Name)
				assert.Equal(t, tt.request.Namespace, request.Namespace)
				assert.NotNil(t, ctx)
				return tt.innerResponse
			})

			tracedHandler := innerHandler.WithTrace(tt.handlerName)

			response := tracedHandler(context.Background(), logger, tt.request, startTime)

			assert.Equal(t, tt.expectCalled, called, "inner handler should be called")
			if tt.validateResponse != nil {
				tt.validateResponse(t, response)
			}
		})
	}
}

func TestFromAdmissionFunc(t *testing.T) {
	logger := testr.New(t)

	t.Run("creates traced handler", func(t *testing.T) {
		called := false
		expectedResponse := AdmissionResponse{
			UID:     types.UID("test-uid"),
			Allowed: true,
		}

		handler := func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
			called = true
			return expectedResponse
		}

		tracedHandler := FromAdmissionFunc("TestFunc", handler)

		request := AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
				Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				Name:      "test-pod",
				Namespace: "default",
				Operation: admissionv1.Create,
				UserInfo: authenticationv1.UserInfo{
					Username: "test-user",
				},
				RequestKind: &metav1.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				},
				RequestResource: &metav1.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "pods",
				},
			},
			GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		}

		response := tracedHandler(context.Background(), logger, request, time.Now())

		assert.True(t, called, "handler should be called")
		assert.Equal(t, expectedResponse.UID, response.UID)
		assert.Equal(t, expectedResponse.Allowed, response.Allowed)
	})
}
