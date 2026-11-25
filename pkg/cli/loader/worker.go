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
	mu         sync.RWMutex
	closed     bool
	closeOnce  sync.Once
	logger     *logrus.Logger
	ctx        context.Context
	cancel     context.CancelFunc
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

func NewWorkerPool(config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		workers:    config.Workers,
		taskQueue:  make(chan LoadTask, config.QueueSize),
		resultChan: make(chan LoadTaskResult, config.QueueSize),
		logger:     config.Logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	for i := 0; i < config.Workers; i++ {
		wp.wg.Add(1)
		go wp.worker(wp.ctx, i)
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
		case task, ok := <-wp.taskQueue:
			if !ok {
				return
			}

			result := wp.processTask(ctx, task)

			select {
			case wp.resultChan <- result:
			case <-ctx.Done():
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
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	if wp.closed {
		wp.logger.Debug("worker pool is closed; ignoring submitted task: ", task.ID)
		return
	}
	var wpDone <-chan struct{}
	if wp.ctx != nil {
		wpDone = wp.ctx.Done()
	}
	select {
	case <-ctx.Done():
		wp.logger.Debug("worker pool is closed; ignoring submitted task: ", task.ID)
		return
	case <-wpDone:
		wp.logger.Debug("worker pool is closed; ignoring submitted task: ", task.ID)
		return
	default:
		select {
		case wp.taskQueue <- task:
			return
		default:
			wp.logger.Debug("task queue is full; ignoring submitted task: ", task.ID)
		}
	}
}

func (wp *WorkerPool) GetResults() <-chan LoadTaskResult {
	return wp.resultChan
}

func (wp *WorkerPool) Cancel() {
	if wp.cancel != nil {
		wp.cancel()
	}
}

func (wp *WorkerPool) Close(cancel context.CancelFunc) {
	wp.closeOnce.Do(func() {
		if cancel != nil {
			cancel()
		}
		wp.Cancel()
		wp.mu.Lock()
		wp.closed = true
		close(wp.taskQueue)
		wp.mu.Unlock()
		wp.wg.Wait()
		close(wp.resultChan)
	})
}
