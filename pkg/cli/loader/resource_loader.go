package loader

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const avgSequentialCallTime = 300 * time.Millisecond

type ClusterLoader struct {
	client          dclient.Interface
	workerPool      *WorkerPool
	resourceOptions ResourceOptions
	logger          *logrus.Logger
	mutex           sync.RWMutex
	closed          bool
}

func LoadResourcesConcurrent(policies []engineapi.GenericPolicy, dClient dclient.Interface, resourceOptions ResourceOptions, showPerformance bool) ([]*unstructured.Unstructured, error) {
	resourceLoader, err := NewClusterLoader(dClient, resourceOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource loader: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer resourceLoader.Close(cancel)
	result, err := resourceLoader.LoadResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load resources: %w", err)
	}

	if showPerformance {
		printLoadingSummary(result.Report)
	}

	return result.Resources, nil
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
		cl.workerPool.SubmitTask(ctx, task)
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

func (cl *ClusterLoader) Close(cancel context.CancelFunc) error {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	if cl.closed {
		return nil
	}

	cl.closed = true

	if cl.workerPool != nil {
		cl.workerPool.Close(cancel)
	}

	return nil
}

func printLoadingSummary(report LoadReport) {
	fmt.Printf("\nResource Loading Performance:\n")
	fmt.Printf("  Duration: %s\n", report.Duration)
	fmt.Printf("  Resources loaded: %d\n", report.SuccessfullyLoaded)
	fmt.Printf("  API calls: %d\n", report.APICallsCount)
	fmt.Printf("  Concurrent workers: %d\n", report.ConcurrentWorkers)

	if len(report.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(report.Errors))
		for _, err := range report.Errors {
			fmt.Printf("    - %s: %s\n", err.ResourceType, err.Error)
		}
	}

	sequentialEstimate := time.Duration(report.APICallsCount) * avgSequentialCallTime
	if sequentialEstimate > report.Duration {
		improvement := float64(sequentialEstimate-report.Duration) / float64(sequentialEstimate) * 100
		fmt.Printf("  Estimated improvement: %.1f%% faster than sequential\n", improvement)
	}
}
