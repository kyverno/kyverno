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
	if obj, quit := queue.Get(); !quit {
		defer queue.Done(obj)
		handleErr(logger, queue, maxRetries, reconcile(obj, r), obj)
		return true
	}
	return false
}

func handleErr(logger logr.Logger, queue workqueue.RateLimitingInterface, maxRetries int, err error, obj interface{}) {
	if err == nil {
		queue.Forget(obj)
	} else if errors.IsNotFound(err) {
		logger.V(4).Info("Dropping request from the queue", "obj", obj, "error", err.Error())
		queue.Forget(obj)
	} else if queue.NumRequeues(obj) < maxRetries {
		logger.V(3).Info("Retrying request", "obj", obj, "error", err.Error())
		queue.AddRateLimited(obj)
	} else {
		logger.Error(err, "Failed to process request", "obj", obj)
		queue.Forget(obj)
	}
}

func reconcile(obj interface{}, r reconcileFunc) error {
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
	return r(k, ns, n)
}
