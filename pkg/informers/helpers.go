package informers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/cache"
)

type informer interface {
	Informer() cache.SharedIndexInformer
}

func StartInformers(ctx context.Context, informers ...informer) {
	for i := range informers {
		go func(informer cache.SharedIndexInformer) {
			informer.Run(ctx.Done())
		}(informers[i].Informer())
	}
}

func WaitForCacheSync(ctx context.Context, logger logr.Logger, informers ...informer) bool {
	var cacheSyncs []cache.InformerSynced
	for i := range informers {
		cacheSyncs = append(cacheSyncs, informers[i].Informer().HasSynced)
	}
	return cache.WaitForCacheSync(ctx.Done(), cacheSyncs...)
}

func StartInformersAndWaitForCacheSync(ctx context.Context, logger logr.Logger, informers ...informer) bool {
	StartInformers(ctx, informers...)
	return WaitForCacheSync(ctx, logger, informers...)
}
