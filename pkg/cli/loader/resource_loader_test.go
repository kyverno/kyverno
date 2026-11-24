package loader

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewClusterLoader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := dclient.NewFakeClient(runtime.NewScheme(), nil)
		_, cancel := context.WithCancel(context.Background())
		loader, err := NewClusterLoader(client, ResourceOptions{
			ResourceTypes: []schema.GroupVersionKind{{Kind: "Pod"}},
			Concurrency:   2,
			BatchSize:     100,
		})
		assert.NoError(t, err)
		assert.NotNil(t, loader)
		loader.Close(cancel)
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

		ctx, cancel := context.WithCancel(context.Background())
		defer loader.Close(cancel)
		result, err := loader.LoadResources(ctx)
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
		ctx, cancel := context.WithCancel(context.Background())
		loader.Close(cancel)
		_, err := loader.LoadResources(ctx)
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
		_, cancel := context.WithCancel(context.Background())
		assert.NoError(t, loader.Close(cancel))
		assert.NoError(t, loader.Close(cancel))
	})
}

func TestClusterLoader_executeTasks(t *testing.T) {
	t.Run("empty tasks", func(t *testing.T) {
		cl := &ClusterLoader{}
		results, err := cl.executeTasks(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, results)
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		cl := &ClusterLoader{
			logger: logrus.New(),
			workerPool: &WorkerPool{
				taskQueue:  make(chan LoadTask, 5),
				resultChan: make(chan LoadTaskResult, 5),
				logger:     logrus.New(),
			},
			resourceOptions: ResourceOptions{
				Timeout: 1 * time.Second,
			},
		}

		tasks := []LoadTask{{ID: "task-0"}}
		results, err := cl.executeTasks(ctx, tasks)
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, results)
	})

	t.Run("timeout returns error when ContinueOnError is false", func(t *testing.T) {
		cl := &ClusterLoader{
			logger: logrus.New(),
			workerPool: &WorkerPool{
				taskQueue:  make(chan LoadTask, 5),
				resultChan: make(chan LoadTaskResult, 5),
				logger:     logrus.New(),
			},
			resourceOptions: ResourceOptions{
				Timeout:         50 * time.Millisecond,
				ContinueOnError: false,
			},
		}

		tasks := []LoadTask{{ID: "task-0"}}
		start := time.Now()
		results, err := cl.executeTasks(context.Background(), tasks)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout waiting for task results")
		assert.Nil(t, results)
		assert.Less(t, elapsed, 500*time.Millisecond)
	})

	t.Run("timeout stops immediately and returns partial results when ContinueOnError is true", func(t *testing.T) {
		resChan := make(chan LoadTaskResult, 5)
		// Provide 1 result for task-0; task-1 and task-2 never send results
		resChan <- LoadTaskResult{TaskID: "task-0"}

		cl := &ClusterLoader{
			logger: logrus.New(),
			workerPool: &WorkerPool{
				taskQueue:  make(chan LoadTask, 5),
				resultChan: resChan,
				logger:     logrus.New(),
			},
			resourceOptions: ResourceOptions{
				Timeout:         60 * time.Millisecond,
				ContinueOnError: true,
			},
		}

		// 3 expected tasks: 1 arrives, 2 time out
		tasks := []LoadTask{{ID: "task-0"}, {ID: "task-1"}, {ID: "task-2"}}
		start := time.Now()
		results, err := cl.executeTasks(context.Background(), tasks)
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "task-0", results[0].TaskID)
		// Must exit on the first timeout (approx 60ms), NOT wait for all 2 missing tasks sequentially (which would take >= 120ms)
		assert.Less(t, elapsed, 110*time.Millisecond)
	})

	t.Run("all tasks succeed and timer resets across results", func(t *testing.T) {
		resChan := make(chan LoadTaskResult, 5)
		resChan <- LoadTaskResult{TaskID: "task-0"}
		resChan <- LoadTaskResult{TaskID: "task-1"}

		cl := &ClusterLoader{
			logger: logrus.New(),
			workerPool: &WorkerPool{
				taskQueue:  make(chan LoadTask, 5),
				resultChan: resChan,
				logger:     logrus.New(),
			},
			resourceOptions: ResourceOptions{
				Timeout:         100 * time.Millisecond,
				ContinueOnError: false,
			},
		}

		tasks := []LoadTask{{ID: "task-0"}, {ID: "task-1"}}
		results, err := cl.executeTasks(context.Background(), tasks)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "task-0", results[0].TaskID)
		assert.Equal(t, "task-1", results[1].TaskID)
	})
}
