package main

import (
	"context"
	"errors"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	metadataclient "k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
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

func StartAdmissionReportsWatcher(ctx context.Context, client metadataclient.Interface) (*resourcesCount, error) {
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

type counter struct {
	count int
}

func (c *counter) Count() int {
	return c.count
}

func StartResourceCounter(ctx context.Context, client metadataclient.Interface, gvr schema.GroupVersionResource, tweakListOptions internalinterfaces.TweakListOptionsFunc) (*counter, error) {
	objs, err := client.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	watcher := &cache.ListWatch{
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			if tweakListOptions != nil {
				tweakListOptions(&options)
			}
			return client.Resource(gvr).Watch(ctx, options)
		},
	}
	watchInterface, err := watchtools.NewRetryWatcher(objs.GetResourceVersion(), watcher)
	if err != nil {
		return nil, err
	}
	w := &counter{
		count: len(objs.Items),
	}
	go func() {
		for event := range watchInterface.ResultChan() {
			switch event.Type {
			case watch.Added:
				w.count = w.count + 1
			case watch.Deleted:
				w.count = w.count - 1
			}
		}
	}()
	return w, nil
}

func StartAdmissionReportsCounter(ctx context.Context, client metadataclient.Interface) (Counter, error) {
	tweakListOptions := func(lo *metav1.ListOptions) {
		lo.LabelSelector = "audit.kyverno.io/source==admission"
	}
	ephrs, err := StartResourceCounter(ctx, client, reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"), tweakListOptions)
	if err != nil {
		return nil, err
	}
	cephrs, err := StartResourceCounter(ctx, client, reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"), tweakListOptions)
	if err != nil {
		return nil, err
	}
	return composite{
		inner: []Counter{ephrs, cephrs},
	}, nil
}

type composite struct {
	inner []Counter
}

func (c composite) Count() int {
	sum := 0
	for _, counter := range c.inner {
		sum += counter.Count()
	}
	return sum
}
