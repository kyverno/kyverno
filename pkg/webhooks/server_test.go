package webhooks

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
