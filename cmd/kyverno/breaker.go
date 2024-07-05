package main

import (
	"context"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	watchtools "github.com/kyverno/kyverno/cmd/kyverno/watch"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	metadataclient "k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
)

type Counter interface {
	Count() (int, bool)
}

type counter struct {
	count        int
	retryWatcher *watchtools.RetryWatcher
}

func (c *counter) Count() (int, bool) {
	return c.count, c.retryWatcher.IsRunning()
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
		count:        len(objs.Items),
		retryWatcher: watchInterface,
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

func (c composite) Count() (int, bool) {
	sum := 0
	for _, counter := range c.inner {
		count, isRunning := counter.Count()
		if !isRunning {
			return 0, false
		}
		sum += count
	}
	return sum, true
}
