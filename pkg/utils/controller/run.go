package controller

import (
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type reconcileFunc func(string, string, string) error

func Run(controllerName string, logger logr.Logger, queue workqueue.RateLimitingInterface, n, maxRetries int, r reconcileFunc, stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) {
	defer runtime.HandleCrash()
	logger.Info("starting ...")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync(controllerName, stopCh, cacheSyncs...) {
		return
	}

	for i := 0; i < n; i++ {
		go wait.Until(func() { worker(logger, queue, maxRetries, r) }, time.Second, stopCh)
	}
	<-stopCh
}

func worker(logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) {
	for processNextWorkItem(logger, queue, maxRetries, r) {
	}
}

func processNextWorkItem(logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) bool {
	if key, quit := queue.Get(); !quit {
		defer queue.Done(key)
		handleErr(logger, queue, maxRetries, reconcile(key.(string), r), key)
		return true
	}
	return false
}

func handleErr(logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, err error, key interface{}) {
	if err == nil {
		queue.Forget(key)
	} else if errors.IsNotFound(err) {
		logger.V(4).Info("Dropping request from the queue", "key", key, "error", err.Error())
		queue.Forget(key)
	} else if queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("Retrying request", "key", key, "error", err.Error())
		queue.AddRateLimited(key)
	} else {
		logger.Error(err, "Failed to process request", "key", key)
		queue.Forget(key)
	}
}

func reconcile(key string, r reconcileFunc) error {
	if namespace, name, err := cache.SplitMetaNamespaceKey(key); err != nil {
		return err
	} else {
		return r(key, namespace, name)
	}
}
