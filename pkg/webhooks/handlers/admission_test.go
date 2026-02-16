package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// mockAdmissionHandler creates a handler that returns a fixed response for testing
func mockAdmissionHandler(response AdmissionResponse) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		return response
	}
}

func TestWithAdmission(t *testing.T) {
	logger := testr.New(t)

	baseAdmissionRequest := &admissionv1.AdmissionRequest{
		UID:       types.UID("test-uid"),
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "test-pod",
		Namespace: "default",
		Operation: admissionv1.Create,
		UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
	}

	testCases := []struct {
		name               string
		requestBody        []byte
		contentType        string
		handlerResponse    AdmissionResponse
		expectedStatusCode int
		expectResponse     bool
		urlParams          map[string]string
	}{
		{
			name: "Valid Admission Request",
			requestBody: func() []byte {
				review := admissionv1.AdmissionReview{
					Request: baseAdmissionRequest,
				}
				body, _ := json.Marshal(review)
				return body
			}(),
			contentType: "application/json",
			handlerResponse: AdmissionResponse{
				Allowed: true,
				UID:     "test-uid",
				Result: &metav1.Status{
					Message: "Pod is allowed",
				},
			},
			expectedStatusCode: http.StatusOK,
			expectResponse:     true,
			urlParams:          map[string]string{"policy": "my-policy"},
		},
		{
			name:               "Empty Request Body",
			requestBody:        nil,
			contentType:        "application/json",
			expectedStatusCode: http.StatusExpectationFailed,
		},
		{
			name:               "Invalid Content-Type",
			requestBody:        []byte(`{}`),
			contentType:        "text/plain",
			expectedStatusCode: http.StatusUnsupportedMediaType,
		},
		{
			name:               "Malformed JSON Body",
			requestBody:        []byte(`{"key": "value"`),
			contentType:        "application/json",
			expectedStatusCode: http.StatusExpectationFailed,
		},
		{
			name:               "Nil Request Field",
			requestBody:        []byte(`{"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1"}`),
			contentType:        "application/json",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Handler Returns Not Allowed",
			requestBody: func() []byte {
				review := admissionv1.AdmissionReview{
					Request: baseAdmissionRequest,
				}
				body, _ := json.Marshal(review)
				return body
			}(),
			contentType: "application/json",
			handlerResponse: AdmissionResponse{
				Allowed: false,
				UID:     "test-uid",
				Result: &metav1.Status{
					Message: "Pod is denied",
					Code:    http.StatusForbidden,
				},
			},
			expectedStatusCode: http.StatusOK,
			expectResponse:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := mockAdmissionHandler(tc.handlerResponse)
			httpHandler := handler.withAdmission(logger)
			req := httptest.NewRequest("POST", "/validate", bytes.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)

			if tc.urlParams != nil {
				params := make(httprouter.Params, 0, len(tc.urlParams))
				for key, value := range tc.urlParams {
					params = append(params, httprouter.Param{Key: key, Value: value})
				}
				ctx := context.WithValue(req.Context(), httprouter.ParamsKey, params)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()

			httpHandler(rr, req)

			assert.Equal(t, tc.expectedStatusCode, rr.Code, "handler returned wrong status code")

			if tc.expectResponse {
				var responseReview admissionv1.AdmissionReview
				err := json.Unmarshal(rr.Body.Bytes(), &responseReview)
				assert.NoError(t, err, "could not unmarshal response body")

				assert.Equal(t, tc.handlerResponse.Allowed, responseReview.Response.Allowed)
				assert.Equal(t, tc.handlerResponse.UID, responseReview.Response.UID)
				assert.Equal(t, tc.handlerResponse.Result.Message, responseReview.Response.Result.Message)
			}
		})
	}
}
