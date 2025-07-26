package loader

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

type ClusterLoader struct {
	client          dclient.Interface
	discovery       discovery.DiscoveryInterface
	workerPool      *WorkerPool
	resourceOptions ResourceOptions
	logger          *logrus.Logger
	mutex           sync.RWMutex
	closed          bool
}

func NewClusterLoader(client dclient.Interface, resourceOptions ResourceOptions) (*ClusterLoader, error) {
	if client == nil {
		return nil, fmt.Errorf("dynamic client cannot be nil")
	}
	resourceOptions.Timeout = 5 * time.Minute
	cl := &ClusterLoader{
		client:          client,
		resourceOptions: resourceOptions,
		logger:          logrus.New(),
	}

	cl.workerPool = NewWorkerPool(WorkerPoolConfig{
		Workers:   resourceOptions.Concurrency,
		QueueSize: resourceOptions.Concurrency * 2,
		Logger:    cl.logger,
	})

	return cl, nil
}

func (cl *ClusterLoader) LoadResources(ctx context.Context) (*ResourceResult, error) {
	startTime := time.Now()

	cl.mutex.RLock()
	if cl.closed {
		cl.mutex.RUnlock()
		return nil, fmt.Errorf("loader is closed")
	}
	cl.mutex.RUnlock()

	if err := cl.validateOptions(cl.resourceOptions); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	tasks := cl.createLoadingTasks()

	results, err := cl.executeTasks(ctx, tasks)
	if err != nil {
		return nil, fmt.Errorf("task execution failed: %w", err)
	}

	finalResult := cl.aggregateResults(results, startTime, cl.resourceOptions.Concurrency)

	return finalResult, nil
}

func (cl *ClusterLoader) validateOptions(options ResourceOptions) error {
	if len(options.ResourceTypes) == 0 {
		return fmt.Errorf("at least one resource type must be specified")
	}

	if options.Concurrency < 1 {
		return fmt.Errorf("concurrency must be greater than 1")
	}

	if options.Concurrency > 32 {
		return fmt.Errorf("concurrency cannot exceed 32")
	}

	if options.BatchSize < 100 {
		return fmt.Errorf("batch size cannot be less than 100")
	}

	if options.BatchSize > 20000 {
		return fmt.Errorf("batch size cannot exceed 20000")
	}

	return nil
}

func (cl *ClusterLoader) createLoadingTasks() []LoadTask {
	var tasks []LoadTask
	taskID := 0
	gvks := cl.resourceOptions.ResourceTypes
	restMapper, err := utils.GetRESTMapper(cl.client, true)
	if err != nil {
		return nil
	}
	for _, gvk := range gvks {
		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil
		}
		gvr := mapping.Resource
		client := cl.client.GetDynamicInterface().Resource(gvr)

		listOptions := metav1.ListOptions{
			Limit: int64(cl.resourceOptions.BatchSize),
		}

		if cl.resourceOptions.Namespace != "" {
			tasks = append(tasks, LoadTask{
				ID:          fmt.Sprintf("task-%d", taskID),
				GVK:         gvk,
				GVR:         gvr,
				Namespace:   cl.resourceOptions.Namespace,
				ListOptions: listOptions,
				Client:      client.Namespace(cl.resourceOptions.Namespace),
			})
		} else {
			tasks = append(tasks, LoadTask{
				ID:          fmt.Sprintf("task-%d", taskID),
				GVK:         gvk,
				GVR:         gvr,
				Namespace:   "",
				ListOptions: listOptions,
				Client:      client,
			})

		}
		taskID++
	}

	return tasks
}

func (cl *ClusterLoader) executeTasks(ctx context.Context, tasks []LoadTask) ([]LoadTaskResult, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	for _, task := range tasks {
		cl.workerPool.SubmitTask(task)
	}

	var results []LoadTaskResult
	expectedResults := len(tasks)

	for i := 0; i < expectedResults; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-cl.workerPool.GetResults():
			results = append(results, result)
		case <-time.After(cl.resourceOptions.Timeout):
			if !cl.resourceOptions.ContinueOnError {
				return nil, fmt.Errorf("timeout waiting for task results")
			}
			break
		}
	}

	return results, nil
}

func (cl *ClusterLoader) aggregateResults(results []LoadTaskResult, startTime time.Time, workers int) *ResourceResult {
	var allResources []*unstructured.Unstructured
	var errors []LoadError
	totalDuration := time.Since(startTime)
	apiCallCount := 0

	for _, result := range results {
		if result.Error != nil {
			errors = append(errors, LoadError{
				ResourceType: result.TaskID,
				Error:        result.Error,
				Retryable:    true,
				Timestamp:    time.Now(),
			})
		} else {
			allResources = append(allResources, result.Resources...)
		}

		if result.APICall {
			apiCallCount++
		}
	}

	return &ResourceResult{
		Resources: allResources,
		Report: LoadReport{
			TotalRequested:     len(results),
			SuccessfullyLoaded: len(allResources),
			Failed:             len(errors),
			Duration:           totalDuration,
			Errors:             errors,
			APICallsCount:      apiCallCount,
			ConcurrentWorkers:  workers,
		},
	}
}

func (cl *ClusterLoader) isClusterScoped(gvr schema.GroupVersionResource) bool {
	clusterScopedResources := map[string]bool{
		"nodes":               true,
		"namespaces":          true,
		"clusterroles":        true,
		"clusterrolebindings": true,
		"persistentvolumes":   true,
	}
	return clusterScopedResources[gvr.Resource]
}

func (cl *ClusterLoader) Close() error {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	if cl.closed {
		return nil
	}

	cl.closed = true

	if cl.workerPool != nil {
		cl.workerPool.Close()
	}

	return nil
}
