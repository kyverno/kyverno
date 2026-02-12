package admissionpolicy

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	testNamespace = "test-ns"
	ns1           = "ns-1"
	ns2           = "ns-2"
)

type mockDClient struct {
	dclient.Interface
	kubeClient kubernetes.Interface
}

func (m *mockDClient) GetKubeClient() kubernetes.Interface {
	return m.kubeClient
}

func TestCustomNamespaceListerGet(t *testing.T) {
	tests := []struct {
		name        string
		nsName      string
		injectError bool
		wantErr     bool
		wantFound   bool
	}{
		{
			name:      "found existing namespace",
			nsName:    testNamespace,
			wantErr:   false,
			wantFound: true,
		},
		{
			name:      "not found non-existent namespace",
			nsName:    "missing-ns",
			wantErr:   true,
			wantFound: false,
		},
		{
			name:        "internal server error",
			nsName:      testNamespace,
			injectError: true,
			wantErr:     true,
			wantFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fake.NewClientset(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
			})

			if tt.injectError {
				k8sClient.PrependReactor("get", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("simulated internal error")
				})
			}

			mockDC := &mockDClient{kubeClient: k8sClient}
			lister := NewCustomNamespaceLister(mockDC)

			got, err := lister.Get(tt.nsName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.nsName, got.Name)
			}
		})
	}
}

func TestCustomNamespaceListerList(t *testing.T) {
	t.Run("list all namespaces success", func(t *testing.T) {
		k8sClient := fake.NewClientset(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns1}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns2}},
		)

		mockDC := &mockDClient{kubeClient: k8sClient}
		lister := NewCustomNamespaceLister(mockDC)

		list, err := lister.List(labels.Everything())
		assert.NoError(t, err)
		assert.Len(t, list, 2)
	})

	t.Run("list namespaces failure", func(t *testing.T) {
		k8sClient := fake.NewClientset()
		
		k8sClient.PrependReactor("list", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("simulated network error")
		})

		mockDC := &mockDClient{kubeClient: k8sClient}
		lister := NewCustomNamespaceLister(mockDC)

		list, err := lister.List(labels.Everything())
		assert.Error(t, err)
		assert.Nil(t, list)
		assert.Equal(t, "simulated network error", err.Error())
	})
}