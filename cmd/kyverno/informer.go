package main

import (
	"reflect"
)

// TODO: eventually move this in an util package
type informer interface {
	Start(stopCh <-chan struct{})
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

func startInformers(stopCh <-chan struct{}, informers ...informer) {
	for i := range informers {
		informers[i].Start(stopCh)
	}
}

func waitForCacheSync(stopCh <-chan struct{}, informers ...informer) {
	for i := range informers {
		informers[i].WaitForCacheSync(stopCh)
	}
}

func startInformersAndWaitForCacheSync(stopCh <-chan struct{}, informers ...informer) {
	startInformers(stopCh, informers...)
	waitForCacheSync(stopCh, informers...)
}
