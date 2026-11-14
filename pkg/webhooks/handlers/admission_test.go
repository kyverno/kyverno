package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// countingReader yields an endless stream of bytes and records how many were read.
type countingReader struct{ read int64 }

func (r *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	r.read += int64(len(p))
	return len(p), nil
}

func TestWithAdmissionRejectsOversizedBody(t *testing.T) {
	called := false
	handler := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return AdmissionResponse{Allowed: true}
	})
	body := &countingReader{}
	req := httptest.NewRequest("POST", "/validate", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.withAdmission(testr.New(t))(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.False(t, called, "inner handler must not run for an oversized body")
	assert.LessOrEqual(t, body.read, maxAdmissionRequestBytes+64*1024, "body must not be read far past the limit")
}

func TestWithAdmissionRejectsContentTypeBeforeReadingBody(t *testing.T) {
	body := &countingReader{}
	req := httptest.NewRequest("POST", "/validate", body)
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	mockAdmissionHandler(AdmissionResponse{}).withAdmission(testr.New(t))(rr, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
	assert.Zero(t, body.read, "body must not be read when Content-Type is invalid")
}

// paddedAdmissionReview returns a valid AdmissionReview body padded with trailing
// JSON whitespace to exactly size bytes.
func paddedAdmissionReview(t *testing.T, size int64) []byte {
	t.Helper()
	review, err := json.Marshal(admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{UID: types.UID("test-uid")}})
	assert.NoError(t, err)
	return append(review, bytes.Repeat([]byte(" "), int(size)-len(review))...)
}

func TestWithAdmissionBodySizeBoundary(t *testing.T) {
	testCases := []struct {
		name               string
		size               int64
		expectedStatusCode int
		expectCalled       bool
	}{
		{name: "at limit", size: maxAdmissionRequestBytes, expectedStatusCode: http.StatusOK, expectCalled: true},
		{name: "one byte over limit", size: maxAdmissionRequestBytes + 1, expectedStatusCode: http.StatusRequestEntityTooLarge},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				called = true
				return AdmissionResponse{Allowed: true}
			})
			req := httptest.NewRequest("POST", "/validate", bytes.NewReader(paddedAdmissionReview(t, tc.size)))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.withAdmission(testr.New(t))(rr, req)

			assert.Equal(t, tc.expectedStatusCode, rr.Code)
			assert.Equal(t, tc.expectCalled, called)
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestWithAdmissionBodyReadError(t *testing.T) {
	called := false
	handler := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return AdmissionResponse{Allowed: true}
	})
	req := httptest.NewRequest("POST", "/validate", failingReader{})
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.withAdmission(testr.New(t))(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.False(t, called, "inner handler must not run when the body cannot be read")
}
