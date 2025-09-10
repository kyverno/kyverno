package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type reconcileFunc func(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error

func Run[T comparable](ctx context.Context, logger logr.Logger, controllerName string, period time.Duration, queue workqueue.TypedRateLimitingInterface[T], n, maxRetries int, r reconcileFunc, routines ...func(context.Context, logr.Logger)) {
	logger.V(2).Info("starting ...")
	defer logger.V(2).Info("stopped")
	var wg sync.WaitGroup
	defer wg.Wait()
	defer runtime.HandleCrash()
	func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer queue.ShutDown()
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(logger logr.Logger) {
				logger.V(4).Info("starting worker")
				defer logger.V(4).Info("worker stopped")
				defer wg.Done()
				wait.UntilWithContext(ctx, func(ctx context.Context) { worker(ctx, logger, controllerName, queue, maxRetries, r) }, period)
			}(logger.WithName("worker").WithValues("id", i))
		}
		for i, routine := range routines {
			wg.Add(1)
			go func(logger logr.Logger, routine func(context.Context, logr.Logger)) {
				logger.V(4).Info("starting routine")
				defer logger.V(4).Info("routine stopped")
				defer wg.Done()
				routine(ctx, logger)
			}(logger.WithName("routine").WithValues("id", i), routine)
		}
		<-ctx.Done()
	}()
	logger.V(4).Info("waiting for workers to terminate ...")
}

func worker[T comparable](ctx context.Context, logger logr.Logger, controllerName string, queue workqueue.TypedRateLimitingInterface[T], maxRetries int, r reconcileFunc) {
	for processNextWorkItem(ctx, logger, controllerName, queue, maxRetries, r) {
	}
}

func processNextWorkItem[T comparable](ctx context.Context, logger logr.Logger, controllerName string, queue workqueue.TypedRateLimitingInterface[T], maxRetries int, r reconcileFunc) bool {
	if obj, quit := queue.Get(); !quit {
		defer queue.Done(obj)
		handleErr(ctx, logger, controllerName, queue, maxRetries, reconcile(ctx, logger, obj, r), obj)
		return true
	}
	return false
}

func handleErr[T comparable](ctx context.Context, logger logr.Logger, controllerName string, queue workqueue.TypedRateLimitingInterface[T], maxRetries int, err error, obj T) {
	metric := metrics.GetControllerMetrics()

	if err == nil {
		queue.Forget(obj)
	} else if errors.IsNotFound(err) {
		logger.V(4).Info("Dropping request from the queue", "obj", obj, "error", err.Error())
		queue.Forget(obj)
	} else if queue.NumRequeues(obj) < maxRetries {
		logger.V(3).Info("Retrying request", "obj", obj, "error", err.Error())
		queue.AddRateLimited(obj)
		if metric != nil {
			metric.RecordRequeueIncrease(ctx, controllerName, queue.NumRequeues(obj))
		}
	} else {
		logger.Error(err, "Failed to process request", "obj", obj)
		queue.Forget(obj)
		if metric != nil {
			metric.RecordQueueDrop(ctx, controllerName)
		}
	}
}

func reconcile(ctx context.Context, logger logr.Logger, obj interface{}, r reconcileFunc) error {
	start := time.Now()
	var k, ns, n string
	if key, ok := obj.(cache.ExplicitKey); ok {
		k = string(key)
	} else {
		k = obj.(string)
		if namespace, name, err := cache.SplitMetaNamespaceKey(k); err != nil {
			return err
		} else {
			ns, n = namespace, name
		}
	}
	logger = logger.WithValues("key", k, "namespace", ns, "name", n)
	logger.V(4).Info("reconciling ...")
	defer func(start time.Time) {
		logger.V(4).Info("done", "duration", time.Since(start).String())
	}(start)
	return r(ctx, logger, k, ns, n)
}
