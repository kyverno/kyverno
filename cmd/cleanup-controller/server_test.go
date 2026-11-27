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
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

type mockMetricsConfig struct {
	metrics.MetricsConfigManager
}

func (m *mockMetricsConfig) Config() config.MetricsConfiguration { return nil }

type mockCertValidator struct {
	valid bool
	err   error
}

func (m mockCertValidator) ValidateCert(context.Context) (bool, error) {
	return m.valid, m.err
}

// buildTestServer wires NewServer the same way cmd/cleanup-controller/main.go does: a
// success-only inner validation handler decorated by WithLegacyPolicyDenial() one layer up
// (see server.go), so any denial observed here comes from the decorator, not the handler.
func buildTestServer(t *testing.T, probes Probes) *httprouter.Router {
	t.Helper()
	successOnly := func(_ context.Context, _ logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
		return handlers.AdmissionResponse{UID: request.UID, Allowed: true}
	}
	tlsProvider := func() ([]byte, []byte, error) { return []byte("cert"), []byte("key"), nil }
	s := NewServer(
		tlsProvider, successOnly, successOnly,
		&mockMetricsConfig{}, webhooks.DebugModeOptions{DumpPayload: false},
		probes, config.NewDefaultConfiguration(false),
	)
	srv, ok := s.(*server)
	require.True(t, ok, "NewServer must return a *server")
	router, ok := srv.server.Handler.(*httprouter.Router)
	require.True(t, ok, "server handler must be an httprouter.Router")
	return router
}

// TestServerLegacyPolicyDenial proves the cleanup-controller's validating route denies legacy
// CleanupPolicy/ClusterCleanupPolicy writes at the mux level (WithLegacyPolicyDenial), including
// the 1.20 behavioural flip: a status-subresource update is now denied, not allowed.
func TestServerLegacyPolicyDenial(t *testing.T) {
	t.Parallel()
	router := buildTestServer(t, runtimeutils.NewRuntime(
		logr.Discard(),
		"",
		informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0).Apps().V1().Deployments(),
		mockCertValidator{valid: true},
	))

	terminatingOld := []byte(`{"metadata":{"name":"ccleanup","deletionTimestamp":"2024-01-01T00:00:00Z","finalizers":["kyverno.io/cleanup"]},"spec":{}}`)
	terminatingNew := []byte(`{"metadata":{"name":"ccleanup","deletionTimestamp":"2024-01-01T00:00:00Z","finalizers":[]},"spec":{}}`)

	tests := []struct {
		name        string
		kind        metav1.GroupVersionKind
		operation   admissionv1.Operation
		subResource string
		oldObject   []byte
		object      []byte
		allowed     bool
	}{
		{
			name: "create ClusterCleanupPolicy denied",
			kind: metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterCleanupPolicy"}, operation: admissionv1.Create,
			object: []byte(`{"metadata":{"name":"ccleanup"},"spec":{}}`), allowed: false,
		},
		{
			name: "top-level update denied",
			kind: metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "CleanupPolicy"}, operation: admissionv1.Update,
			oldObject: []byte(`{"metadata":{"name":"cleanup"},"spec":{"a":1}}`), object: []byte(`{"metadata":{"name":"cleanup"},"spec":{"a":2}}`), allowed: false,
		},
		{
			// This is PR 1's headline behavioural flip: the legacy status writer that used to
			// register "status" as a transitionally allowed subresource is gone, so this now
			// denies rather than succeeds.
			name: "status subresource update denied",
			kind: metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "CleanupPolicy"}, operation: admissionv1.Update, subResource: "status",
			oldObject: []byte(`{"metadata":{"name":"cleanup"},"status":{"ready":false}}`), object: []byte(`{"metadata":{"name":"cleanup"},"status":{"ready":true}}`), allowed: false,
		},
		{
			name: "delete allowed",
			kind: metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterCleanupPolicy"}, operation: admissionv1.Delete,
			object: []byte(`{"metadata":{"name":"ccleanup"},"spec":{}}`), allowed: true,
		},
		{
			name: "finalizer removal on terminating object allowed",
			kind: metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterCleanupPolicy"}, operation: admissionv1.Update,
			oldObject: terminatingOld, object: terminatingNew, allowed: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := &admissionv1.AdmissionRequest{
				UID: "legacy-denial", Operation: test.operation, SubResource: test.subResource,
				Kind:      test.kind,
				Object:    runtime.RawExtension{Raw: test.object},
				OldObject: runtime.RawExtension{Raw: test.oldObject},
			}
			body, err := json.Marshal(admissionv1.AdmissionReview{TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"}, Request: request})
			require.NoError(t, err)
			httpRequest := httptest.NewRequest(http.MethodPost, config.CleanupValidatingWebhookServicePath, bytes.NewReader(body))
			httpRequest.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httpRequest)
			require.Equal(t, http.StatusOK, response.Code)
			var review admissionv1.AdmissionReview
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &review))
			require.NotNil(t, review.Response)
			assert.Equal(t, request.UID, review.Response.UID)
			assert.Equal(t, test.allowed, review.Response.Allowed)
		})
	}
}

func TestServerProbes(t *testing.T) {
	t.Parallel()
	deploymentInformer := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0).Apps().V1().Deployments()
	tests := []struct {
		name             string
		certValid        bool
		informerReady    bool
		expectedReady    int
		expectedLiveness int
	}{
		{
			name:             "healthy certs and informers",
			certValid:        true,
			informerReady:    true,
			expectedReady:    http.StatusOK,
			expectedLiveness: http.StatusOK,
		},
		{
			name:             "invalid cert fails readiness",
			certValid:        false,
			informerReady:    true,
			expectedReady:    http.StatusInternalServerError,
			expectedLiveness: http.StatusOK,
		},
		{
			name:             "unhealthy informer fails readiness",
			certValid:        true,
			informerReady:    false,
			expectedReady:    http.StatusInternalServerError,
			expectedLiveness: http.StatusOK,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			probes := runtimeutils.NewRuntime(
				logr.Discard(),
				"",
				deploymentInformer,
				mockCertValidator{valid: test.certValid},
				func() bool { return test.informerReady },
			)
			router := buildTestServer(t, probes)

			readyResp := httptest.NewRecorder()
			router.ServeHTTP(readyResp, httptest.NewRequest(http.MethodGet, config.ReadinessServicePath, nil))
			assert.Equal(t, test.expectedReady, readyResp.Code)

			liveResp := httptest.NewRecorder()
			router.ServeHTTP(liveResp, httptest.NewRequest(http.MethodGet, config.LivenessServicePath, nil))
			assert.Equal(t, test.expectedLiveness, liveResp.Code)
		})
	}
}
