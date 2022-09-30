package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type reconcileFunc func(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error

func Run(ctx context.Context, controllerName string, logger logr.Logger, queue workqueue.RateLimitingInterface, n, maxRetries int, r reconcileFunc, cacheSyncs ...cache.InformerSynced) {
	logger.Info("starting ...")
	defer runtime.HandleCrash()
	defer logger.Info("stopped")
	var wg sync.WaitGroup
	func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer queue.ShutDown()
		if !cache.WaitForNamedCacheSync(controllerName, ctx.Done(), cacheSyncs...) {
			return
		}
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(logger logr.Logger) {
				logger.Info("starting worker")
				defer wg.Done()
				defer logger.Info("worker stopped")
				wait.UntilWithContext(ctx, func(ctx context.Context) { worker(ctx, logger, queue, maxRetries, r) }, time.Second)
			}(logger.WithValues("id", i))
		}
		<-ctx.Done()
	}()
	logger.Info("waiting for workers to terminate ...")
	wg.Wait()
}

func worker(ctx context.Context, logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) {
	for processNextWorkItem(ctx, logger, queue, maxRetries, r) {
	}
}

func processNextWorkItem(ctx context.Context, logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) bool {
	if obj, quit := queue.Get(); !quit {
		defer queue.Done(obj)
		handleErr(logger, queue, maxRetries, reconcile(ctx, logger, obj, r), obj)
		return true
	}
	return false
}

func handleErr(logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, err error, obj interface{}) {
	if err == nil {
		queue.Forget(obj)
	} else if errors.IsNotFound(err) {
		logger.Info("Dropping request from the queue", "obj", obj, "error", err.Error())
		queue.Forget(obj)
	} else if queue.NumRequeues(obj) < maxRetries {
		logger.Info("Retrying request", "obj", obj, "error", err.Error())
		queue.AddRateLimited(obj)
	} else {
		logger.Error(err, "Failed to process request", "obj", obj)
		queue.Forget(obj)
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
	logger.Info("reconciling ...")
	defer logger.Info("done", time.Since(start))
	return r(ctx, logger, k, ns, n)
}
