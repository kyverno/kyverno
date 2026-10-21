package breaker

import (
	"context"
	"sync"
	"time"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	watchtools "github.com/kyverno/kyverno/cmd/kyverno/watch"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	"github.com/kyverno/kyverno/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	k8swatch "k8s.io/apimachinery/pkg/watch"
	metadataclient "k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
)

type retryWatcher interface {
	ResultChan() <-chan k8swatch.Event
	IsRunning() bool
	Stop()
}

type resourceUIDGetter interface {
	GetUID() types.UID
}

type Counter interface {
	Count() (int, bool)
}

type counter struct {
	lock         sync.RWMutex
	entries      sets.Set[types.UID]
	retryWatcher retryWatcher
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

func (c *counter) consume(events <-chan k8swatch.Event) {
	for event := range events {
		getter, ok := event.Object.(resourceUIDGetter)
		if !ok {
			continue
		}
		switch event.Type {
		case k8swatch.Added, k8swatch.Modified:
			c.Record(getter.GetUID())
		case k8swatch.Deleted:
			c.Forget(getter.GetUID())
		}
	}
}

// resync lists the resource and replaces the entries and watcher the counter
// reports on with a watcher started from the returned resource version.
func (c *counter) resync(ctx context.Context, client metadataclient.Interface, gvr schema.GroupVersionResource, tweakListOptions internalinterfaces.TweakListOptionsFunc) (retryWatcher, error) {
	listOptions := metav1.ListOptions{}
	if tweakListOptions != nil {
		tweakListOptions(&listOptions)
	}
	objs, err := client.Resource(gvr).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	watcher := &cache.ListWatch{
		WatchFunc: func(options metav1.ListOptions) (k8swatch.Interface, error) {
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
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries = entries
	c.retryWatcher = watchInterface
	return watchInterface, nil
}

func StartResourceCounter(ctx context.Context, client metadataclient.Interface, gvr schema.GroupVersionResource, tweakListOptions internalinterfaces.TweakListOptionsFunc) (*counter, error) {
	c := &counter{}
	watchInterface, err := c.resync(ctx, client, gvr, tweakListOptions)
	if err != nil {
		return nil, err
	}
	logger := logging.WithName("resource-counter").WithValues("resource", gvr.Resource)
	go func() {
		for {
			c.consume(watchInterface.ResultChan())
			// The watcher stopped for good, the counter reports itself as not
			// running until a fresh List gives it a usable resource version.
			logger.Info("resource counter watcher stopped, restarting")
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			restarted, err := c.resync(ctx, client, gvr, tweakListOptions)
			if err != nil {
				logger.Error(err, "failed to restart resource counter watcher, retrying...")
				continue
			}
			watchInterface = restarted
		}
	}()
	return c, nil
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

func StartBackgroundReportsCounter(ctx context.Context, client metadataclient.Interface) (Counter, error) {
	tweakListOptions := func(lo *metav1.ListOptions) {
		lo.LabelSelector = "audit.kyverno.io/source==background-scan"
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
