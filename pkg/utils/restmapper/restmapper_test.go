package restmapper

import (
	"context"
	"io"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
)

// mockDClient implements dclient.Interface for testing
type mockDClient struct {
	kubeClient kubernetes.Interface
}

func (m *mockDClient) GetKubeClient() kubernetes.Interface {
	return m.kubeClient
}

func (m *mockDClient) Discovery() dclient.IDiscovery {
	return nil
}

func (m *mockDClient) GetEventsInterface() eventsv1.EventsV1Interface {
	return nil
}

func (m *mockDClient) GetDynamicInterface() dynamic.Interface {
	return nil
}

func (m *mockDClient) SetDiscovery(discoveryClient dclient.IDiscovery) {
}

func (m *mockDClient) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return nil, nil
}

func (m *mockDClient) GetResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDClient) PatchResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDClient) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, nil
}

func (m *mockDClient) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool, options metav1.DeleteOptions) error {
	return nil
}

func (m *mockDClient) CreateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDClient) UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDClient) UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDClient) ApplyResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDClient) ApplyStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string) (*unstructured.Unstructured, error) {
	return nil, nil
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
