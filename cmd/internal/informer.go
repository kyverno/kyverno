package internal

import (
	"context"
	"reflect"
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

func WaitForCacheSync(ctx context.Context, informers ...informer) bool {
	ret := true
	for i := range informers {
		for _, result := range informers[i].WaitForCacheSync(ctx.Done()) {
			ret = ret && result
		}
	}
	return ret
}

func CheckCacheSync[T comparable](status map[T]bool) bool {
	ret := true
	for _, s := range status {
		ret = ret && s
	}
	return ret
}

func StartInformersAndWaitForCacheSync(ctx context.Context, informers ...informer) bool {
	StartInformers(ctx, informers...)
	return WaitForCacheSync(ctx, informers...)
}
