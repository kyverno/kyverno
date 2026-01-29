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

func TestGetRESTMapper(t *testing.T) {
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

