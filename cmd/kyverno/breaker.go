package main

import (
	"context"
	"errors"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	metadataclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type Counter interface {
	Count() int
}

type resourcesCount struct {
	store cache.Store
}

func (c *resourcesCount) Count() int {
	return len(c.store.List())
}

func StartAdmissionReportsWatcher(ctx context.Context, client metadataclient.UpstreamInterface) (*resourcesCount, error) {
	gvr := reportsv1.SchemeGroupVersion.WithResource("ephemeralreports")
	todo := context.TODO()
	tweakListOptions := func(lo *metav1.ListOptions) {
		lo.LabelSelector = "audit.kyverno.io/source==admission"
	}
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				tweakListOptions(&options)
				return client.Resource(gvr).Namespace(metav1.NamespaceAll).List(todo, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				tweakListOptions(&options)
				return client.Resource(gvr).Namespace(metav1.NamespaceAll).Watch(todo, options)
			},
		},
		&metav1.PartialObjectMetadata{},
		resyncPeriod,
		cache.Indexers{},
	)
	err := informer.SetTransform(func(in any) (any, error) {
		{
			in := in.(*metav1.PartialObjectMetadata)
			return &metav1.PartialObjectMetadata{
				TypeMeta: in.TypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:         in.Name,
					GenerateName: in.GenerateName,
					Namespace:    in.Namespace,
				},
			}, nil
		}
	})
	if err != nil {
		return nil, err
	}
	go func() {
		informer.Run(todo.Done())
	}()
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, errors.New("failed to sync cache")
	}
	return &resourcesCount{
		store: informer.GetStore(),
	}, nil
}
