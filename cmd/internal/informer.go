package internal

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
)

type startable interface {
	Start(stopCh <-chan struct{})
}

type informer interface {
	startable
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

func StartInformers[T startable](ctx context.Context, informers ...T) {
	for i := range informers {
		informers[i].Start(ctx.Done())
	}
}

func WaitForCacheSync(ctx context.Context, logger logr.Logger, informers ...informer) bool {
	ret := true
	for i := range informers {
		for t, result := range informers[i].WaitForCacheSync(ctx.Done()) {
			if !result {
				logger.Error(nil, "failed to wait for cache sync", "type", t)
			}
			ret = ret && result
		}
	}
	return ret
}

func CheckCacheSync[T comparable](logger logr.Logger, status map[T]bool) bool {
	ret := true
	for t, result := range status {
		if !result {
			logger.Error(nil, "failed to wait for cache sync", "type", t)
		}
		ret = ret && result
	}
	return ret
}

func StartInformersAndWaitForCacheSync(ctx context.Context, logger logr.Logger, informers ...informer) bool {
	StartInformers(ctx, informers...)
	return WaitForCacheSync(ctx, logger, informers...)
}
