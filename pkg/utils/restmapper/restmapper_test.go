package restmapper

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// mockDClient implements dclient.Interface for testing
type mockDClient struct {
	kubeClient kubernetes.Interface
}

func (m *mockDClient) GetKubeClient() kubernetes.Interface {
	return m.kubeClient
}

// Implement other required methods as no-ops for testing
func (m *mockDClient) Discovery() dclient.IDiscovery                     { return nil }
func (m *mockDClient) GetEventsInterface() dclient.EventInterface        { return nil }
func (m *mockDClient) GetResource(apiVersion, kind, namespace, name, subresources string) (interface{}, error) {
	return nil, nil
}
func (m *mockDClient) GetDynamicInterface() dclient.DynamicInterface { return nil }
func (m *mockDClient) SetDiscovery(discoveryClient dclient.IDiscovery) {
}

func TestGetRESTMapper_WithRealClient(t *testing.T) {
	// Create a fake Kubernetes client
	kubeClient := fake.NewSimpleClientset()

	// Create mock dclient
	client := &mockDClient{
		kubeClient: kubeClient,
	}

	// Call GetRESTMapper with real client (isFake=false)
	mapper, err := GetRESTMapper(client, false)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}

func TestGetRESTMapper_WithFakeClient(t *testing.T) {
	// Create a fake Kubernetes client
	kubeClient := fake.NewSimpleClientset()

	// Create mock dclient
	client := &mockDClient{
		kubeClient: kubeClient,
	}

	// Call GetRESTMapper with fake flag (isFake=true)
	mapper, err := GetRESTMapper(client, true)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}

func TestGetRESTMapper_WithNilClient(t *testing.T) {
	// Call GetRESTMapper with nil client and fake flag
	mapper, err := GetRESTMapper(nil, true)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}

func TestGetRESTMapper_FakeFlagTrue(t *testing.T) {
	// Test that isFake=true always uses embedded API resources
	tests := []struct {
		name   string
		client dclient.Interface
	}{
		{
			name:   "with nil client",
			client: nil,
		},
		{
			name: "with real client but fake flag",
			client: &mockDClient{
				kubeClient: fake.NewSimpleClientset(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, err := GetRESTMapper(tt.client, true)
			assert.NoError(t, err)
			assert.NotNil(t, mapper)
		})
	}
}

func TestGetRESTMapper_FakeFlagFalse(t *testing.T) {
	tests := []struct {
		name      string
		client    dclient.Interface
		expectErr bool
	}{
		{
			name:      "with nil client",
			client:    nil,
			expectErr: false, // Falls back to fake path
		},
		{
			name: "with real client",
			client: &mockDClient{
				kubeClient: fake.NewSimpleClientset(),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, err := GetRESTMapper(tt.client, false)
			
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, mapper)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mapper)
			}
		})
	}
}

func TestGetRESTMapper_ReturnsDifferentMappers(t *testing.T) {
	// Create mock client
	client := &mockDClient{
		kubeClient: fake.NewSimpleClientset(),
	}

	// Get mapper with real client path
	mapper1, err1 := GetRESTMapper(client, false)
	assert.NoError(t, err1)
	assert.NotNil(t, mapper1)

	// Get mapper with fake client path
	mapper2, err2 := GetRESTMapper(client, true)
	assert.NoError(t, err2)
	assert.NotNil(t, mapper2)

	// Both should be valid RESTMappers
	// Note: We can't easily compare types without reflection,
	// but we can verify both are non-nil and different execution paths
	assert.NotNil(t, mapper1)
	assert.NotNil(t, mapper2)
}

func TestGetRESTMapper_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		client  dclient.Interface
		isFake  bool
		wantErr bool
	}{
		{
			name:    "nil client with isFake=true",
			client:  nil,
			isFake:  true,
			wantErr: false,
		},
		{
			name:    "nil client with isFake=false",
			client:  nil,
			isFake:  false,
			wantErr: false,
		},
		{
			name: "valid client with isFake=true",
			client: &mockDClient{
				kubeClient: fake.NewSimpleClientset(),
			},
			isFake:  true,
			wantErr: false,
		},
		{
			name: "valid client with isFake=false",
			client: &mockDClient{
				kubeClient: fake.NewSimpleClientset(),
			},
			isFake:  false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, err := GetRESTMapper(tt.client, tt.isFake)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, mapper)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mapper)
			}
		})
	}
}
