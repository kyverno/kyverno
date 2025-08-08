package loader

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewClusterLoader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := dclient.NewFakeClient(runtime.NewScheme(), nil)
		loader, err := NewClusterLoader(client, ResourceOptions{
			ResourceTypes: []schema.GroupVersionKind{{Kind: "Pod"}},
			Concurrency:   2,
			BatchSize:     100,
		})
		assert.NoError(t, err)
		assert.NotNil(t, loader)
		loader.Close()
	})

	t.Run("nil client", func(t *testing.T) {
		_, err := NewClusterLoader(nil, ResourceOptions{})
		assert.Error(t, err)
	})
}

func TestClusterLoader_LoadResources(t *testing.T) {
	t.Run("basic load", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
		}

		client, _ := dclient.NewFakeClient(runtime.NewScheme(), nil, obj)
		loader, _ := NewClusterLoader(client, ResourceOptions{
			ResourceTypes: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				},
			},
			Concurrency: 2,
			BatchSize:   100, // Must be >= 100 per your validation
			Namespace:   "default",
			Timeout:     5 * time.Minute,
		})
		defer loader.Close()

		result, err := loader.LoadResources(context.Background())
		assert.NoError(t, err)
		if assert.Len(t, result.Resources, 1) {
			assert.Equal(t, "test-pod", result.Resources[0].GetName())
		}
	})

	t.Run("closed loader", func(t *testing.T) {
		client, _ := dclient.NewFakeClient(runtime.NewScheme(), nil)
		loader, _ := NewClusterLoader(client, ResourceOptions{
			ResourceTypes: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				},
			},
			Concurrency: 2,
			BatchSize:   100,
			Timeout:     5 * time.Minute,
		})
		loader.Close()

		_, err := loader.LoadResources(context.Background())
		assert.Error(t, err)
	})
}

func TestClusterLoader_Options(t *testing.T) {
	t.Run("invalid options", func(t *testing.T) {
		loader := &ClusterLoader{}
		err := loader.validateOptions(ResourceOptions{})
		assert.Error(t, err)
	})

	t.Run("valid options", func(t *testing.T) {
		loader := &ClusterLoader{}
		err := loader.validateOptions(ResourceOptions{
			ResourceTypes: []schema.GroupVersionKind{{Kind: "Pod"}},
			Concurrency:   2,
			BatchSize:     100,
		})
		assert.NoError(t, err)
	})
}

func TestClusterLoader_Tasks(t *testing.T) {
	t.Run("create tasks", func(t *testing.T) {
		client, _ := dclient.NewFakeClient(runtime.NewScheme(), nil)
		loader := &ClusterLoader{
			resourceOptions: ResourceOptions{
				ResourceTypes: []schema.GroupVersionKind{{Kind: "Pod"}},
				Namespace:     "test",
			},
			client: client,
		}

		tasks := loader.createLoadingTasks()
		assert.Len(t, tasks, 1)
		assert.Equal(t, "test", tasks[0].Namespace)
	})
}

func TestClusterLoader_Close(t *testing.T) {
	t.Run("double close", func(t *testing.T) {
		client, _ := dclient.NewFakeClient(runtime.NewScheme(), nil)
		loader, _ := NewClusterLoader(client, ResourceOptions{
			ResourceTypes: []schema.GroupVersionKind{{Kind: "Pod"}},
		})

		assert.NoError(t, loader.Close())
		assert.NoError(t, loader.Close())
	})
}
