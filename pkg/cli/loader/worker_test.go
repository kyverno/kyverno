package loader

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubeTesting "k8s.io/client-go/testing"
)

// createFakeClient returns a dynamic fake client populated with N unstructured objects.
func createFakeClient(objects ...runtime.Object) *fake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return fake.NewSimpleDynamicClient(scheme, objects...)
}

func TestWorkerPool_SubmitAndReceiveResults(t *testing.T) {
	logger := logrus.New()
	wp := NewWorkerPool(WorkerPoolConfig{
		Workers:   2,
		QueueSize: 5,
		Logger:    logger,
	})

	// Create a fake unstructured object
	obj := &unstructured.Unstructured{}
	obj.SetName("test-object")
	obj.SetNamespace("default")
	obj.SetKind("ConfigMap")
	obj.SetAPIVersion("v1")

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}

	client := createFakeClient(obj)
	resourceClient := client.Resource(gvr).Namespace("default")

	task := LoadTask{
		ID:        "task-1",
		GVK:       gvk,
		GVR:       gvr,
		Namespace: "default",
		ListOptions: v1.ListOptions{
			Limit: 100,
		},
		Client: resourceClient,
	}

	wp.SubmitTask(context.Background(), task)

	select {
	case result := <-wp.GetResults():
		assert.NoError(t, result.Error)
		assert.Equal(t, "task-1", result.TaskID)
		assert.Len(t, result.Resources, 1)
		assert.Equal(t, "test-object", result.Resources[0].GetName())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for task result")
	}
	_, cancel := context.WithCancel(context.Background())
	wp.Close(cancel)
}

func TestWorkerPool_Pagination(t *testing.T) {
	logger := logrus.New()
	wp := NewWorkerPool(WorkerPoolConfig{
		Workers:   1,
		QueueSize: 10,
		Logger:    logger,
	})

	// Create multiple fake objects
	var objs []runtime.Object
	for i := 0; i < 250; i++ {
		obj := &unstructured.Unstructured{}
		obj.SetName("obj-" + string(rune(i)))
		obj.SetNamespace("default")
		obj.SetKind("ConfigMap")
		obj.SetAPIVersion("v1")
		objs = append(objs, obj)
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
	client := createFakeClient(objs...)

	resourceClient := client.Resource(gvr)

	task := LoadTask{
		ID:        "task-paged",
		GVK:       gvk,
		GVR:       gvr,
		Namespace: "",
		ListOptions: v1.ListOptions{
			Limit: 100,
		},
		Client: resourceClient,
	}
	ctx, cancel := context.WithCancel(context.Background())
	wp.SubmitTask(ctx, task)

	select {
	case result := <-wp.GetResults():
		assert.NoError(t, result.Error)
		assert.Equal(t, "task-paged", result.TaskID)
		assert.Len(t, result.Resources, 250)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for paginated result")
	}

	wp.Close(cancel)
}

func TestWorkerPool_ErrorHandling(t *testing.T) {
	logger := logrus.New()
	wp := NewWorkerPool(WorkerPoolConfig{
		Workers:   1,
		QueueSize: 1,
		Logger:    logger,
	})

	var callCount int32
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "failures"}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Failure"}

	// Create a client that always fails
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		gvr: "FailureList",
	})
	client.PrependReactor("list", "*", func(action kubeTesting.Action) (bool, runtime.Object, error) {
		atomic.AddInt32(&callCount, 1)
		return true, nil, errors.New("API error")
	})

	resourceClient := client.Resource(gvr)

	task := LoadTask{
		ID:        "error-task",
		GVK:       gvk,
		GVR:       gvr,
		Namespace: "",
		ListOptions: v1.ListOptions{
			Limit: 50,
		},
		Client: resourceClient,
	}
	ctx, cancel := context.WithCancel(context.Background())
	wp.SubmitTask(ctx, task)

	select {
	case result := <-wp.GetResults():
		assert.Error(t, result.Error)
		assert.Equal(t, "error-task", result.TaskID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error result")
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
	wp.Close(cancel)
}

func TestWorkerPool_GracefulShutdown(t *testing.T) {
	logger := logrus.New()
	wp := NewWorkerPool(WorkerPoolConfig{
		Workers:   1,
		QueueSize: 1,
		Logger:    logger,
	})
	ctx, cancel := context.WithCancel(context.Background())
	wp.Close(cancel)

	// Submitting after close should not panic or deadlock
	task := LoadTask{
		ID: "noop-task",
	}
	wp.SubmitTask(ctx, task)

	select {
	case <-time.After(500 * time.Millisecond):
	}
}

func TestNewWorkerPool_Initialization(t *testing.T) {
	logger := logrus.New()
	wp := NewWorkerPool(WorkerPoolConfig{
		Workers:   3,
		QueueSize: 10,
		Logger:    logger,
	})
	_, cancel := context.WithCancel(context.Background())
	defer wp.Close(cancel)

	assert.NotNil(t, wp.taskQueue)
	assert.NotNil(t, wp.resultChan)
	assert.Equal(t, 3, wp.workers)
	assert.Equal(t, 10, cap(wp.taskQueue))
}

func TestWorkerPool_ResultChannelClosing(t *testing.T) {
	logger := logrus.New()
	wp := NewWorkerPool(WorkerPoolConfig{
		Workers:   1,
		QueueSize: 1,
		Logger:    logger,
	})
	_, cancel := context.WithCancel(context.Background())
	wp.Close(cancel)

	// Verify result channel is closed after processing
	_, ok := <-wp.GetResults()
	assert.False(t, ok, "Result channel should be closed after Close()")
}
