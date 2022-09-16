package controller

import (
	"errors"

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
	AddKeyedEventHandlers(logger, informer, queue, MetaNamespaceKey)
}

func AddKeyedEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, keyFunc keyFunc) {
	AddEventHandlers(informer, AddFunc(logger, queue, keyFunc), UpdateFunc(logger, queue, keyFunc), DeleteFunc(logger, queue, keyFunc))
}

func Enqueue(logger logr.Logger, queue workqueue.RateLimitingInterface, obj interface{}, keyFunc keyFunc) error {
	if key, err := keyFunc(obj); err != nil {
		logger.Error(err, "failed to compute key name", "obj", obj)
		return err
	} else {
		queue.Add(key)
		return nil
	}
}

func MetaNamespaceKey(obj interface{}) (interface{}, error) {
	return cache.MetaNamespaceKeyFunc(obj)
}

func Explicit[K any](keyFunc func(K) cache.ExplicitKey) keyFunc {
	return func(obj interface{}) (interface{}, error) {
		if obj == nil {
			return nil, errors.New("obj is nil")
		}
		if key, ok := obj.(K); !ok {
			return nil, errors.New("obj cannot be converted")
		} else {
			return keyFunc(key), nil
		}
	}
}

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
