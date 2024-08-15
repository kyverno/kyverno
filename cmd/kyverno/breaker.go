package main

import (
	"context"
	"sync"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	watchtools "github.com/kyverno/kyverno/cmd/kyverno/watch"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	metadataclient "k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
)

type resourceUIDGetter interface {
	GetUID() types.UID
}

type Counter interface {
	Count() (int, bool)
}

type counter struct {
	lock         sync.RWMutex
	entries      sets.Set[types.UID]
	retryWatcher *watchtools.RetryWatcher
}

func (c *counter) Record(uid types.UID) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries.Insert(uid)
}

func (c *counter) Forget(uid types.UID) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries.Delete(uid)
}

func (c *counter) Count() (int, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.entries.Len(), c.retryWatcher.IsRunning()
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
	entries := sets.New[types.UID]()
	for _, entry := range objs.Items {
		entries.Insert(entry.GetUID())
	}
	w := &counter{
		entries:      entries,
		retryWatcher: watchInterface,
	}
	go func() {
		for event := range watchInterface.ResultChan() {
			getter, ok := event.Object.(resourceUIDGetter)
			if ok {
				switch event.Type {
				case watch.Added:
					w.Record(getter.GetUID())
				case watch.Modified:
					w.Record(getter.GetUID())
				case watch.Deleted:
					w.Forget(getter.GetUID())
				}
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
