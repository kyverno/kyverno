package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

type mockHandler struct{}

func (m *mockHandler) Execute(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
	return admissionv1.AdmissionResponse{Allowed: true}
}

type mockDiscovery struct {
	dclient.IDiscovery
}

func (m *mockDiscovery) DiscoveryCache() cache.SharedInformer {
	return nil
}

type mockMetricsConfig struct {
	metrics.MetricsConfigManager
}

func (m *mockMetricsConfig) Config() config.MetricsConfiguration {
	return nil
}

type mockRBLister struct{}

func (m *mockRBLister) List(selector labels.Selector) (ret []*rbacv1.RoleBinding, err error) {
	return nil, nil
}
func (m *mockRBLister) RoleBindings(namespace string) rbacv1listers.RoleBindingNamespaceLister {
	return &mockRBNsLister{}
}

type mockRBNsLister struct{}

func (m *mockRBNsLister) List(selector labels.Selector) (ret []*rbacv1.RoleBinding, err error) {
	return nil, nil
}
func (m *mockRBNsLister) Get(name string) (*rbacv1.RoleBinding, error) { return nil, nil }

type mockCRBLister struct{}

func (m *mockCRBLister) List(selector labels.Selector) (ret []*rbacv1.ClusterRoleBinding, err error) {
	return nil, nil
}
func (m *mockCRBLister) Get(name string) (*rbacv1.ClusterRoleBinding, error) { return nil, nil }

type mockRuntime struct {
	isGoingDown bool
}

func (m *mockRuntime) IsGoingDown() bool                { return m.isGoingDown }
func (m *mockRuntime) IsLive(ctx context.Context) bool  { return true }
func (m *mockRuntime) IsReady(ctx context.Context) bool { return true }
func (m *mockRuntime) IsDebug() bool                    { return false }
func (m *mockRuntime) IsRollingUpdate() bool            { return false }

type mockDeleteClient struct {
	deletedItems []string
	shouldError  bool
}

func (m *mockDeleteClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	if m.shouldError {
		return fmt.Errorf("injected deletion error")
	}
	m.deletedItems = append(m.deletedItems, name)
	return nil
}

type mockDeleteCollectionClient struct {
	deleteCollectionCalled bool
	shouldError            bool
}

func (m *mockDeleteCollectionClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	if m.shouldError {
		return fmt.Errorf("injected collection deletion error")
	}
	m.deleteCollectionCalled = true
	return nil
}

func (m *mockDeleteCollectionClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return nil
}
func (m *mockDeleteCollectionClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (interface{}, error) {
	return nil, nil
}
func (m *mockDeleteCollectionClient) List(ctx context.Context, opts metav1.ListOptions) (interface{}, error) {
	return nil, nil
}
func (m *mockDeleteCollectionClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (m *mockDeleteCollectionClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (interface{}, error) {
	return nil, nil
}

func TestNewServer(t *testing.T) {
	ctx := context.TODO()
	dummyHandler := &mockHandler{}
	cfg := config.NewDefaultConfiguration(false)
	metricsMgr := &mockMetricsConfig{}
	discoveryMock := &mockDiscovery{}
	runtimeMock := &mockRuntime{isGoingDown: false}
	mwcClient := &mockDeleteCollectionClient{}
	vwcClient := &mockDeleteCollectionClient{}
	leaseClient := &mockDeleteClient{}
	rbLister := &mockRBLister{}
	crbLister := &mockCRBLister{}

	pHandlers := PolicyHandlers{Mutation: dummyHandler, Validation: dummyHandler}
	rHandlers := ResourceHandlers{
		MutatingPolicies: dummyHandler, NamespacedMutatingPolicies: dummyHandler,
		ValidatingPolicies: dummyHandler, NamespacedValidatingPolicies: dummyHandler,
		GeneratingPolicies: dummyHandler, NamespacedGeneratingPolicies: dummyHandler,
		ImageVerificationPolicies: dummyHandler, ImageVerificationPoliciesMutation: dummyHandler,
		Mutation: dummyHandler, Validation: dummyHandler,
	}
	eHandlers := ExceptionHandlers{Validation: dummyHandler}
	celHandlers := CELExceptionHandlers{Validation: dummyHandler}
	gcHandlers := GlobalContextHandlers{Validation: dummyHandler}
	debugOpts := DebugModeOptions{DumpPayload: false}
	tlsProvider := func() ([]byte, []byte, error) { return []byte("cert"), []byte("key"), nil }

	s := NewServer(
		ctx, pHandlers, rHandlers, eHandlers, celHandlers, gcHandlers,
		cfg, metricsMgr, debugOpts, tlsProvider,
		mwcClient, vwcClient, leaseClient, runtimeMock,
		rbLister, crbLister, discoveryMock, "localhost", 8080,
	)

	assert.NotNil(t, s)
	srvStruct, ok := s.(*server)
	assert.True(t, ok)
	assert.Equal(t, "[localhost]:8080", srvStruct.server.Addr)
}

// buildTestServer wires NewServer with mock handlers, mirroring TestNewServer, and returns the
// underlying httprouter so route registration can be asserted against the real server.
func buildTestServer(t *testing.T, backgroundServiceAccountName ...string) *httprouter.Router {
	t.Helper()
	ctx := context.TODO()
	dummyHandler := &mockHandler{}
	cfg := config.NewDefaultConfiguration(false)
	metricsMgr := &mockMetricsConfig{}
	discoveryMock := &mockDiscovery{}
	runtimeMock := &mockRuntime{isGoingDown: false}
	mwcClient := &mockDeleteCollectionClient{}
	vwcClient := &mockDeleteCollectionClient{}
	leaseClient := &mockDeleteClient{}
	rbLister := &mockRBLister{}
	crbLister := &mockCRBLister{}

	pHandlers := PolicyHandlers{Mutation: dummyHandler, Validation: dummyHandler}
	rHandlers := ResourceHandlers{
		MutatingPolicies: dummyHandler, NamespacedMutatingPolicies: dummyHandler,
		ValidatingPolicies: dummyHandler, NamespacedValidatingPolicies: dummyHandler,
		GeneratingPolicies: dummyHandler, NamespacedGeneratingPolicies: dummyHandler,
		ImageVerificationPolicies: dummyHandler, ImageVerificationPoliciesMutation: dummyHandler,
		Mutation: dummyHandler, Validation: dummyHandler,
	}
	eHandlers := ExceptionHandlers{Validation: dummyHandler}
	celHandlers := CELExceptionHandlers{Validation: dummyHandler}
	gcHandlers := GlobalContextHandlers{Validation: dummyHandler}
	debugOpts := DebugModeOptions{DumpPayload: false}
	tlsProvider := func() ([]byte, []byte, error) { return []byte("cert"), []byte("key"), nil }

	s := NewServer(
		ctx, pHandlers, rHandlers, eHandlers, celHandlers, gcHandlers,
		cfg, metricsMgr, debugOpts, tlsProvider,
		mwcClient, vwcClient, leaseClient, runtimeMock,
		rbLister, crbLister, discoveryMock, "localhost", 8080, backgroundServiceAccountName...,
	)
	srv, ok := s.(*server)
	require.True(t, ok, "NewServer must return a *server")
	router, ok := srv.server.Handler.(*httprouter.Router)
	require.True(t, ok, "server handler must be an httprouter.Router")
	return router
}

// TestServer_NamespacedImageValidatingPolicyRoutesAreServed verifies the webhook server serves the
// NamespacedImageValidatingPolicy admission paths. The webhook controller registers these policies'
// webhooks at /nivpol/mutate and /nivpol/validate (pkg/controllers/webhook/controller.go), but the
// webhook server only registered /ivpol/mutate and /ivpol/validate, so the API server posts to a
// path the server does not serve and gets "the server could not find the requested resource"
// (HTTP 404), meaning a NamespacedImageValidatingPolicy never runs at admission.
func TestServer_NamespacedImageValidatingPolicyRoutesAreServed(t *testing.T) {
	router := buildTestServer(t)

	// All four image-validating-policy webhook paths registered by the webhook controller must be
	// served by the webhook server, otherwise the API server's webhook call 404s.
	for _, path := range []string{
		"/ivpol/mutate/sample",    // ImageValidatingPolicy (cluster-scoped)
		"/ivpol/validate/sample",  // ImageValidatingPolicy (cluster-scoped)
		"/nivpol/mutate/sample",   // NamespacedImageValidatingPolicy
		"/nivpol/validate/sample", // NamespacedImageValidatingPolicy
	} {
		handle, _, _ := router.Lookup(http.MethodPost, path)
		assert.NotNilf(t, handle, "POST %s is not routed by the webhook server; the API server gets a 404 and the policy never runs", path)
	}

	// Concretely, this is the 404 the API server reports as "the server could not find the
	// requested resource" for every NamespacedImageValidatingPolicy admission today.
	req := httptest.NewRequest(http.MethodPost, "/nivpol/mutate/sample", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.NotEqualf(t, http.StatusNotFound, rr.Code, "POST /nivpol/mutate returned %d (API server: the server could not find the requested resource)", rr.Code)
}

func TestServerStop(t *testing.T) {
	tests := []struct {
		name                 string
		isGoingDown          bool
		expectCleanup        bool
		expectedLeaseDeletes []string
	}{
		{name: "Runtime is NOT going down", isGoingDown: false, expectCleanup: false},
		{name: "Runtime IS going down", isGoingDown: true, expectCleanup: true, expectedLeaseDeletes: []string{"kyvernopre-lock", "kyverno-health"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := &mockRuntime{isGoingDown: tt.isGoingDown}
			mockLeaseClient := &mockDeleteClient{deletedItems: []string{}}
			mockMWCClient := &mockDeleteCollectionClient{}
			mockVWCClient := &mockDeleteCollectionClient{}

			s := &server{
				server:      &http.Server{},
				runtime:     mockRuntime,
				leaseClient: mockLeaseClient,
				mwcClient:   mockMWCClient,
				vwcClient:   mockVWCClient,
			}

			s.Stop()

			if tt.expectCleanup {
				assert.True(t, mockMWCClient.deleteCollectionCalled)
				assert.True(t, mockVWCClient.deleteCollectionCalled)
				assert.ElementsMatch(t, tt.expectedLeaseDeletes, mockLeaseClient.deletedItems)
			} else {
				assert.False(t, mockMWCClient.deleteCollectionCalled)
				assert.False(t, mockVWCClient.deleteCollectionCalled)
				assert.Empty(t, mockLeaseClient.deletedItems)
			}
		})
	}
}

func TestServerStopWithErrors(t *testing.T) {
	mockRuntime := &mockRuntime{isGoingDown: true}
	mockLeaseClient := &mockDeleteClient{shouldError: true}
	mockMWCClient := &mockDeleteCollectionClient{shouldError: true}
	mockVWCClient := &mockDeleteCollectionClient{shouldError: true}

	s := &server{
		server:      &http.Server{},
		runtime:     mockRuntime,
		leaseClient: mockLeaseClient,
		mwcClient:   mockMWCClient,
		vwcClient:   mockVWCClient,
	}

	assert.NotPanics(t, func() {
		s.Stop()
	})
}

func TestServerRunDoesNotPanic(t *testing.T) {
	s := &server{server: &http.Server{Addr: ":0"}}
	assert.NotPanics(t, func() { s.Run(); time.Sleep(10 * time.Millisecond) })
}

func TestServerGenerationLabelProtection(t *testing.T) {
	t.Parallel()
	background := config.KyvernoUserName("custom-background")
	router := buildTestServer(t, background)
	resource := func(labels map[string]string, value string) []byte {
		object := map[string]any{
			"apiVersion": "v1", "kind": "Secret",
			"metadata": map[string]any{"name": "target", "namespace": "outside-trigger-scope", "labels": labels},
			"data":     map[string]string{"value": value},
		}
		data, err := json.Marshal(object)
		require.NoError(t, err)
		return data
	}
	labels := map[string]string{"generate.kyverno.io/policy-name": "policy", "app.kubernetes.io/managed-by": "kyverno"}
	tests := []struct {
		name                 string
		username             string
		operation            admissionv1.Operation
		newLabels, oldLabels map[string]string
		allowed              bool
	}{
		{name: "tenant cannot create labels", username: "system:serviceaccount:tenant:editor", operation: admissionv1.Create, newLabels: labels},
		{name: "unrelated installation account cannot create labels", username: config.KyvernoUserName("unrelated"), operation: admissionv1.Create, newLabels: labels},
		{name: "unrelated installation account cannot remove labels", username: config.KyvernoUserName("unrelated"), operation: admissionv1.Update, oldLabels: labels},
		{name: "configured background account creates labels", username: background, operation: admissionv1.Create, newLabels: labels, allowed: true},
		{name: "configured admission account creates labels", username: config.KyvernoUserName(config.KyvernoServiceAccountName()), operation: admissionv1.Create, newLabels: labels, allowed: true},
		{name: "background account name prefix is not trusted", username: background + "-other", operation: admissionv1.Create, newLabels: labels},
		{name: "ordinary downstream edit retains labels", username: "system:serviceaccount:tenant:editor", operation: admissionv1.Update, oldLabels: labels, newLabels: labels, allowed: true},
		{name: "standalone managed-by edit remains allowed", username: "system:serviceaccount:tenant:editor", operation: admissionv1.Create, newLabels: map[string]string{"app.kubernetes.io/managed-by": "kyverno"}, allowed: true},
		{name: "clone-source edit remains allowed", username: "system:serviceaccount:tenant:editor", operation: admissionv1.Create, newLabels: map[string]string{"generate.kyverno.io/clone-source": ""}, allowed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := &admissionv1.AdmissionRequest{
				UID: "metadata-request", Namespace: "outside-trigger-scope", Operation: test.operation,
				Kind:     metav1.GroupVersionKind{Version: "v1", Kind: "Secret"},
				Resource: metav1.GroupVersionResource{Version: "v1", Resource: "secrets"},
				UserInfo: authenticationv1.UserInfo{Username: test.username},
				Object:   runtime.RawExtension{Raw: resource(test.newLabels, "ZWRpdGVk")},
			}
			if test.operation == admissionv1.Update {
				request.OldObject = runtime.RawExtension{Raw: resource(test.oldLabels, "b3JpZ2luYWw=")}
			}
			body, err := json.Marshal(admissionv1.AdmissionReview{TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"}, Request: request})
			require.NoError(t, err)
			httpRequest := httptest.NewRequest(http.MethodPost, config.GenerationLabelProtectionWebhookServicePath, bytes.NewReader(body))
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
