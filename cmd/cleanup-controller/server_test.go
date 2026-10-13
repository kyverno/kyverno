package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/auth"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCleanupAdmissionAuthentication(t *testing.T) {
	called := false
	handler := func(context.Context, logr.Logger, handlers.AdmissionRequest, time.Time) handlers.AdmissionResponse {
		called = true
		return handlers.AdmissionResponse{Allowed: true}
	}
	s := NewServer(
		func() ([]byte, []byte, error) { return nil, nil, nil },
		handler, handler, nil, webhooks.DebugModeOptions{}, probes{},
		config.NewDefaultConfiguration(false), auth.NewReceiver(nil, "", 443),
	).(*server)
	review, err := json.Marshal(admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{UID: "cleanup-test"}})
	require.NoError(t, err)
	for _, path := range []string{config.CleanupValidatingWebhookServicePath, config.TtlValidatingWebhookServicePath} {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(review))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		var result admissionv1.AdmissionReview
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.NotNil(t, result.Response)
		require.False(t, result.Response.Allowed)
		require.Equal(t, types.UID("cleanup-test"), result.Response.UID)
	}
	require.False(t, called)
	for _, path := range []string{config.LivenessServicePath, config.ReadinessServicePath} {
		response := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, response.Code)
	}
}
