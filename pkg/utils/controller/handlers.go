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
	keyFunc    func(interface{}) (interface{}, error)
)

func AddEventHandlers(informer cache.SharedInformer, a addFunc, u updateFunc, d deleteFunc) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a,
		UpdateFunc: u,
		DeleteFunc: d,
	})
}

func AddDefaultEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface) {
	AddEventHandlers(informer, AddFunc(logger, queue, cache.MetaNamespaceKeyFunc), UpdateFunc(logger, queue, cache.MetaNamespaceKeyFunc), DeleteFunc(logger, queue, cache.MetaNamespaceKeyFunc))
}

func AddExplicitEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, explicit explicitFunc) {
	AddEventHandlers(informer, Add(logger, queue), Update(logger, queue), Delete(logger, queue))
}

func Enqueue(logger logr.Logger, queue workqueue.RateLimitingInterface, obj interface{}, keyFunc keyFunc) {
	if key, err := keyFunc(obj); err != nil {
		logger.Error(err, "failed to compute key name", "obj", obj)
	} else {
		queue.Add(key)
	}
}

MetaNamespaceKey
func AddFunc(logger logr.Logger, queue workqueue.RateLimitingInterface, keyFunc keyFunc) addFunc {
	return func(obj interface{}) {
		Enqueue(logger, queue, obj, keyFunc)
	}
}

func UpdateFunc(logger logr.Logger, queue workqueue.RateLimitingInterface, keyFunc keyFunc) updateFunc {
	return func(_, obj interface{}) {
		Enqueue(logger, queue, obj, keyFunc)
	}
}

func DeleteFunc(logger logr.Logger, queue workqueue.RateLimitingInterface, keyFunc keyFunc) deleteFunc {
	return func(obj interface{}) {
		Enqueue(logger, queue, kubeutils.GetObjectWithTombstone(obj), keyFunc)
	}
}
