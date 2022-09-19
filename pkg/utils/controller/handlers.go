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

func AddKeyedEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, parseKey keyFunc) {
	AddEventHandlers(informer, AddFunc(logger, queue, parseKey), UpdateFunc(logger, queue, parseKey), DeleteFunc(logger, queue, parseKey))
}

func AddDefaultEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface) {
	AddKeyedEventHandlers(logger, informer, queue, MetaNamespaceKey)
}

func AddExplicitEventHandlers[K any](logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, parseKey func(K) cache.ExplicitKey) {
	AddKeyedEventHandlers(logger, informer, queue, ExplicitKey(parseKey))
}

func Enqueue(logger logr.Logger, queue workqueue.RateLimitingInterface, obj interface{}, parseKey keyFunc) error {
	if key, err := parseKey(obj); err != nil {
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

func ExplicitKey[K any](parseKey func(K) cache.ExplicitKey) keyFunc {
	return func(obj interface{}) (interface{}, error) {
		if obj == nil {
			return nil, errors.New("obj is nil")
		}
		if key, ok := obj.(K); !ok {
			return nil, errors.New("obj cannot be converted")
		} else {
			return parseKey(key), nil
		}
	}
}

func AddFunc(logger logr.Logger, queue workqueue.RateLimitingInterface, parseKey keyFunc) addFunc {
	return func(obj interface{}) {
		Enqueue(logger, queue, obj, parseKey)
	}
}

func UpdateFunc(logger logr.Logger, queue workqueue.RateLimitingInterface, parseKey keyFunc) updateFunc {
	return func(_, obj interface{}) {
		Enqueue(logger, queue, obj, parseKey)
	}
}

func DeleteFunc(logger logr.Logger, queue workqueue.RateLimitingInterface, parseKey keyFunc) deleteFunc {
	return func(obj interface{}) {
		Enqueue(logger, queue, kubeutils.GetObjectWithTombstone(obj), parseKey)
	}
}
