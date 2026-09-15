package loader

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type WorkerPool struct {
	workers    int
	taskQueue  chan LoadTask
	resultChan chan LoadTaskResult
	wg         sync.WaitGroup
	logger     *logrus.Logger
	done       chan struct{}
}

type WorkerPoolConfig struct {
	Workers   int
	QueueSize int
	Logger    *logrus.Logger
}

type LoadTask struct {
	ID          string
	GVK         schema.GroupVersionKind
	GVR         schema.GroupVersionResource
	Namespace   string
	ListOptions metav1.ListOptions
	Client      dynamic.ResourceInterface
}

type LoadTaskResult struct {
	TaskID    string
	Resources []*unstructured.Unstructured
	Error     error
	Duration  time.Duration
	APICall   bool
}

func NewWorkerPool(ctx context.Context, config WorkerPoolConfig) *WorkerPool {
	wp := &WorkerPool{
		workers:    config.Workers,
		taskQueue:  make(chan LoadTask, config.QueueSize),
		resultChan: make(chan LoadTaskResult, config.QueueSize),
		logger:     config.Logger,
		done:       make(chan struct{}),
	}

	for i := 0; i < config.Workers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}

	return wp
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()
	wp.logger.WithField("worker_id", id).Debug("Worker started")

	for {
		select {
		case <-ctx.Done():
			wp.logger.WithField("worker_id", id).Debug("Worker stopping")
			return
		case <-wp.done:
			wp.logger.WithField("worker_id", id).Debug("Worker stopping")
			return
		case task := <-wp.taskQueue:
			result := wp.processTask(ctx, task)

			select {
			case wp.resultChan <- result:
			case <-ctx.Done():
				return
			case <-wp.done:
				return
			}
		}
	}
}

func (wp *WorkerPool) processTask(ctx context.Context, task LoadTask) LoadTaskResult {
	start := time.Now()
	result := LoadTaskResult{TaskID: task.ID}

	var allResources []*unstructured.Unstructured
	continueToken := ""

	opts := task.ListOptions.DeepCopy()
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	opts.Limit = limit

	for {
		opts.Continue = continueToken

		list, err := task.Client.List(ctx, *opts)
		if err != nil {
			result.Error = err
			break
		}

		for i := range list.Items {
			allResources = append(allResources, &list.Items[i])
		}

		continueToken = list.GetContinue()
		if continueToken == "" {
			break
		}
	}

	result.Resources = allResources
	result.Duration = time.Since(start)
	return result
}

func (wp *WorkerPool) SubmitTask(ctx context.Context, task LoadTask) {
	select {
	case <-ctx.Done():
		wp.logger.Debug("worker pool is closed; ignoring submitted task: ", task.ID)
	case <-wp.done:
		wp.logger.Debug("worker pool is closed; ignoring submitted task: ", task.ID)
	case wp.taskQueue <- task:
	}
}

func (wp *WorkerPool) GetResults() <-chan LoadTaskResult {
	return wp.resultChan
}

func (wp *WorkerPool) Close(cancel context.CancelFunc) {
	cancel()
	close(wp.done)
	wp.wg.Wait()
	close(wp.resultChan)
}
