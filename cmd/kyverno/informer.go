package main

import (
	"context"
	"reflect"
)

// TODO: eventually move this in an util package
type startable interface {
	Start(stopCh <-chan struct{})
}

type informer interface {
	startable
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

func startInformers[T startable](ctx context.Context, informers ...T) {
	for i := range informers {
		informers[i].Start(ctx.Done())
	}
}

func waitForCacheSync(ctx context.Context, informers ...informer) bool {
	ret := true
	for i := range informers {
		for _, result := range informers[i].WaitForCacheSync(ctx.Done()) {
			ret = ret && result
		}
	}
	return ret
}

func checkCacheSync[T comparable](status map[T]bool) bool {
	ret := true
	for _, s := range status {
		ret = ret && s
	}
	return ret
}

func startInformersAndWaitForCacheSync(ctx context.Context, informers ...informer) bool {
	startInformers(ctx, informers...)
	return waitForCacheSync(ctx, informers...)
}
