package controller

import (
	"github.com/go-logr/logr"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type (
	addFunc    func(interface{})
	updateFunc func(interface{}, interface{})
	deleteFunc func(interface{})
)

func AddEventHandlers(informer cache.SharedInformer, a addFunc, u updateFunc, d deleteFunc) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a,
		UpdateFunc: u,
		DeleteFunc: d,
	})
}

func AddDefaultEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface) {
	AddEventHandlers(informer, Add(logger, queue), Update(logger, queue), Delete(logger, queue))
}

func Enqueue(logger logr.Logger, queue workqueue.RateLimitingInterface, obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		logger.Error(err, "failed to compute key name")
	} else {
		queue.Add(key)
	}
}

func Add(logger logr.Logger, queue workqueue.RateLimitingInterface) addFunc {
	return func(obj interface{}) {
		Enqueue(logger, queue, obj)
	}
}

func Update(logger logr.Logger, queue workqueue.RateLimitingInterface) updateFunc {
	return func(_, obj interface{}) {
		Enqueue(logger, queue, obj)
	}
}

func Delete(logger logr.Logger, queue workqueue.RateLimitingInterface) deleteFunc {
	return func(obj interface{}) {
		Enqueue(logger, queue, kubeutils.GetObjectWithTombstone(obj))
	}
}
