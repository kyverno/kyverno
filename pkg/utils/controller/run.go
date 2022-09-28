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

type reconcileFunc func(logger logr.Logger, key string, namespace string, name string) error

func Run(controllerName string, logger logr.Logger, queue workqueue.RateLimitingInterface, n, maxRetries int, r reconcileFunc, stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) {
	logger.Info("starting ...")
	defer runtime.HandleCrash()
	defer logger.Info("stopped")
	var wg sync.WaitGroup
	func() {
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		defer queue.ShutDown()
		if !cache.WaitForNamedCacheSync(controllerName, stopCh, cacheSyncs...) {
			return
		}
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(logger logr.Logger) {
				logger.Info("starting worker")
				defer wg.Done()
				defer logger.Info("worker stopped")
				wait.Until(func() { worker(logger, queue, maxRetries, r) }, time.Second, ctx.Done())
			}(logger.WithValues("id", i))
		}
		<-stopCh
	}()
	logger.Info("waiting for workers to terminate ...")
	wg.Wait()
}

func worker(logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) {
	for processNextWorkItem(logger, queue, maxRetries, r) {
	}
}

func processNextWorkItem(logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) bool {
	if obj, quit := queue.Get(); !quit {
		defer queue.Done(obj)
		handleErr(logger, queue, maxRetries, reconcile(logger, obj, r), obj)
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

func reconcile(logger logr.Logger, obj interface{}, r reconcileFunc) error {
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
	return r(logger, k, ns, n)
}
