package loader

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

type ClusterLoader struct {
	client     dynamic.Interface
	discovery  discovery.DiscoveryInterface
	workerPool *WorkerPool
	logger     *logrus.Logger
	config     ClusterLoaderConfig
	mutex      sync.RWMutex
	closed     bool
}

type ClusterLoaderConfig struct {
	DefaultConcurrency int
	DefaultBatchSize   int
	DefaultTimeout     time.Duration
	DefaultMaxRetries  int
}

func NewClusterLoader(client dynamic.Interface, config ClusterLoaderConfig) (*ClusterLoader, error) {
	if client == nil {
		return nil, fmt.Errorf("dynamic client cannot be nil")
	}

	if config.DefaultConcurrency <= 0 {
		config.DefaultConcurrency = 4
	}
	if config.DefaultBatchSize <= 0 {
		config.DefaultBatchSize = 100
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.DefaultMaxRetries <= 0 {
		config.DefaultMaxRetries = 3
	}

	cl := &ClusterLoader{
		client: client,
		config: config,
		logger: logrus.New(),
	}

	cl.workerPool = NewWorkerPool(WorkerPoolConfig{
		Workers:   config.DefaultConcurrency,
		QueueSize: config.DefaultConcurrency * 2,
		Logger:    cl.logger,
	})

	return cl, nil
}

func (cl *ClusterLoader) LoadResources(ctx context.Context, options ResourceOptions) (*ResourceResult, error) {
	startTime := time.Now()

	cl.mutex.RLock()
	if cl.closed {
		cl.mutex.RUnlock()
		return nil, fmt.Errorf("loader is closed")
	}
	cl.mutex.RUnlock()

	options = cl.applyDefaults(options)

	if err := cl.validateOptions(options); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	tasks := cl.createLoadingTasks(options.ResourceTypes, options)

	results, err := cl.executeTasks(ctx, tasks, options)
	if err != nil {
		return nil, fmt.Errorf("task execution failed: %w", err)
	}

	finalResult := cl.aggregateResults(results, startTime, options.Concurrency)

	return finalResult, nil
}

func (cl *ClusterLoader) applyDefaults(options ResourceOptions) ResourceOptions {
	if options.Concurrency <= 0 {
		options.Concurrency = cl.config.DefaultConcurrency
	}
	if options.BatchSize <= 0 {
		options.BatchSize = cl.config.DefaultBatchSize
	}
	if options.Timeout <= 0 {
		options.Timeout = cl.config.DefaultTimeout
	}
	if options.MaxRetries <= 0 {
		options.MaxRetries = cl.config.DefaultMaxRetries
	}
	return options
}

func (cl *ClusterLoader) validateOptions(options ResourceOptions) error {
	if len(options.ResourceTypes) == 0 {
		return fmt.Errorf("at least one resource type must be specified")
	}

	if options.Concurrency > 32 {
		return fmt.Errorf("concurrency cannot exceed 32")
	}

	if options.BatchSize > 2000 {
		return fmt.Errorf("batch size cannot exceed 2000")
	}

	return nil
}

func (cl *ClusterLoader) createLoadingTasks(gvks []schema.GroupVersionKind, options ResourceOptions) []LoadTask {
	var tasks []LoadTask
	taskID := 0

	for _, gvk := range gvks {
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: cl.pluralizeKind(gvk.Kind),
		}

		client := cl.client.Resource(gvr)

		listOptions := metav1.ListOptions{
			Limit: int64(options.BatchSize),
		}

		if options.LabelSelector != "" {
			listOptions.LabelSelector = options.LabelSelector
		}

		if options.FieldSelector != "" {
			listOptions.FieldSelector = options.FieldSelector
		}

		if options.Namespace != "" {
			tasks = append(tasks, LoadTask{
				ID:          fmt.Sprintf("task-%d", taskID),
				GVK:         gvk,
				GVR:         gvr,
				Namespace:   options.Namespace,
				ListOptions: listOptions,
				Client:      client.Namespace(options.Namespace),
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

func (cl *ClusterLoader) executeTasks(ctx context.Context, tasks []LoadTask, options ResourceOptions) ([]LoadTaskResult, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	if options.Concurrency != cl.workerPool.workers {
		cl.workerPool.Close()
		cl.workerPool = NewWorkerPool(WorkerPoolConfig{
			Workers:   options.Concurrency,
			QueueSize: options.Concurrency * 2,
			Logger:    cl.logger,
		})
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
		case <-time.After(options.Timeout):
			if !options.ContinueOnError {
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

func (cl *ClusterLoader) pluralizeKind(kind string) string {
	kind = strings.ToLower(kind)
	if strings.HasSuffix(kind, "s") {
		return kind + "es"
	}
	if strings.HasSuffix(kind, "y") {
		return strings.TrimSuffix(kind, "y") + "ies"
	}
	return kind + "s"
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
