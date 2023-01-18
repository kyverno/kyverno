package controller

import (
	"errors"
	"time"

	"github.com/go-logr/logr"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type (
	addFunc             = addFuncT[interface{}]
	updateFunc          = updateFuncT[interface{}]
	deleteFunc          = deleteFuncT[interface{}]
	addFuncT[T any]     func(T)
	updateFuncT[T any]  func(T, T)
	deleteFuncT[T any]  func(T)
	keyFunc             = keyFuncT[interface{}, interface{}]
	keyFuncT[T, U any]  func(T) (U, error)
	EnqueueFunc         = EnqueueFuncT[interface{}]
	EnqueueFuncT[T any] func(T) error
)

func AddEventHandlers(informer cache.SharedInformer, a addFunc, u updateFunc, d deleteFunc) {
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a,
		UpdateFunc: u,
		DeleteFunc: func(obj interface{}) {
			d(kubeutils.GetObjectWithTombstone(obj))
		},
	})
}

func AddEventHandlersT[T any](informer cache.SharedInformer, a addFuncT[T], u updateFuncT[T], d deleteFuncT[T]) {
	AddEventHandlers(
		informer,
		func(obj interface{}) { a(obj.(T)) },
		func(old, obj interface{}) { u(old.(T), obj.(T)) },
		func(obj interface{}) { d(obj.(T)) },
	)
}

func AddKeyedEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, parseKey keyFunc) EnqueueFunc {
	enqueueFunc := LogError(logger, Parse(parseKey, Queue(queue)))
	AddEventHandlers(informer, AddFunc(logger, enqueueFunc), UpdateFunc(logger, enqueueFunc), DeleteFunc(logger, enqueueFunc))
	return enqueueFunc
}

func AddKeyedEventHandlersT[K metav1.Object](logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, parseKey keyFuncT[K, interface{}]) EnqueueFuncT[K] {
	enqueueFunc := LogError(logger, Parse(parseKey, Queue(queue)))
	AddEventHandlersT(informer, AddFuncT(logger, enqueueFunc), UpdateFuncT(logger, enqueueFunc), DeleteFuncT(logger, enqueueFunc))
	return enqueueFunc
}

func AddDelayedKeyedEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, delay time.Duration, parseKey keyFunc) EnqueueFunc {
	enqueueFunc := LogError(logger, Parse(parseKey, QueueAfter(queue, delay)))
	AddEventHandlers(informer, AddFunc(logger, enqueueFunc), UpdateFunc(logger, enqueueFunc), DeleteFunc(logger, enqueueFunc))
	return enqueueFunc
}

func AddDefaultEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface) EnqueueFunc {
	return AddKeyedEventHandlers(logger, informer, queue, MetaNamespaceKey)
}

func AddDefaultEventHandlersT[K metav1.Object](logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface) EnqueueFuncT[K] {
	return AddKeyedEventHandlersT(logger, informer, queue, MetaNamespaceKeyT[K])
}

func AddDelayedDefaultEventHandlers(logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, delay time.Duration) EnqueueFunc {
	return AddDelayedKeyedEventHandlers(logger, informer, queue, delay, MetaNamespaceKey)
}

func AddExplicitEventHandlers[K any](logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, parseKey func(K) cache.ExplicitKey) EnqueueFunc {
	return AddKeyedEventHandlers(logger, informer, queue, ExplicitKey(parseKey))
}

func AddDelayedExplicitEventHandlers[K any](logger logr.Logger, informer cache.SharedInformer, queue workqueue.RateLimitingInterface, delay time.Duration, parseKey func(K) cache.ExplicitKey) EnqueueFunc {
	return AddDelayedKeyedEventHandlers(logger, informer, queue, delay, ExplicitKey(parseKey))
}

func LogError[K any](logger logr.Logger, inner EnqueueFuncT[K]) EnqueueFuncT[K] {
	return func(obj K) error {
		err := inner(obj)
		if err != nil {
			logger.Error(err, "failed to compute key name", "obj", obj)
		}
		return err
	}
}

func Parse[K, L any](parseKey keyFuncT[K, L], inner EnqueueFuncT[L]) EnqueueFuncT[K] {
	return func(obj K) error {
		if key, err := parseKey(obj); err != nil {
			return err
		} else {
			return inner(key)
		}
	}
}

func Queue(queue workqueue.RateLimitingInterface) EnqueueFunc {
	return func(obj interface{}) error {
		queue.Add(obj)
		return nil
	}
}

func QueueAfter(queue workqueue.RateLimitingInterface, delay time.Duration) EnqueueFunc {
	return func(obj interface{}) error {
		queue.AddAfter(obj, delay)
		return nil
	}
}

func MetaNamespaceKey(obj interface{}) (interface{}, error) {
	return cache.MetaNamespaceKeyFunc(obj)
}

func MetaNamespaceKeyT[T any](obj T) (interface{}, error) {
	return MetaNamespaceKey(obj)
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

func AddFunc(logger logr.Logger, enqueue EnqueueFunc) addFunc {
	return func(obj interface{}) {
		if err := enqueue(obj); err != nil {
			logger.Error(err, "failed to enqueue object", "obj", obj)
		}
	}
}

func UpdateFunc(logger logr.Logger, enqueue EnqueueFunc) updateFunc {
	return func(old, obj interface{}) {
		oldMeta := old.(metav1.Object)
		objMeta := obj.(metav1.Object)
		if oldMeta.GetResourceVersion() != objMeta.GetResourceVersion() {
			if err := enqueue(obj); err != nil {
				logger.Error(err, "failed to enqueue object", "obj", obj)
			}
		}
	}
}

func DeleteFunc(logger logr.Logger, enqueue EnqueueFunc) deleteFunc {
	return func(obj interface{}) {
		if err := enqueue(obj); err != nil {
			logger.Error(err, "failed to enqueue object", "obj", obj)
		}
	}
}

func AddFuncT[K metav1.Object](logger logr.Logger, enqueue EnqueueFuncT[K]) addFuncT[K] {
	return func(obj K) {
		if err := enqueue(obj); err != nil {
			logger.Error(err, "failed to enqueue object", "obj", obj)
		}
	}
}

func UpdateFuncT[K metav1.Object](logger logr.Logger, enqueue EnqueueFuncT[K]) updateFuncT[K] {
	return func(old, obj K) {
		if old.GetResourceVersion() != obj.GetResourceVersion() {
			if err := enqueue(obj); err != nil {
				logger.Error(err, "failed to enqueue object", "obj", obj)
			}
		}
	}
}

func DeleteFuncT[K metav1.Object](logger logr.Logger, enqueue EnqueueFuncT[K]) deleteFuncT[K] {
	return func(obj K) {
		if err := enqueue(obj); err != nil {
			logger.Error(err, "failed to enqueue object", "obj", obj)
		}
	}
}
