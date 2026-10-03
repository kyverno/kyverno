package resource

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"gotest.tools/v3/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
)

var (
	configMapsGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	configMapGVK  = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
)

type startedWatch struct {
	resourceVersion string
	watcher         *watch.RaceFreeFakeWatcher
}

// fakeCluster serves configmaps from whatever list is currently set and hands
// out every watch it creates on the watches channel.
type fakeCluster struct {
	*dynamicfake.FakeDynamicClient

	mu        sync.Mutex
	list      *unstructured.UnstructuredList
	failLists int
	watches   chan startedWatch
}

func newFakeCluster() *fakeCluster {
	c := &fakeCluster{
		FakeDynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			runtime.NewScheme(),
			map[schema.GroupVersionResource]string{configMapsGVR: "ConfigMapList"},
		),
		watches: make(chan startedWatch, 8),
	}
	c.PrependReactor("list", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.failLists > 0 {
			c.failLists--
			return true, nil, errors.New("apiserver unavailable")
		}
		return true, c.list.DeepCopy(), nil
	})
	c.PrependWatchReactor("configmaps", func(action k8stesting.Action) (bool, watch.Interface, error) {
		w := watch.NewRaceFreeFake()
		c.watches <- startedWatch{
			resourceVersion: action.(k8stesting.WatchAction).GetWatchRestrictions().ResourceVersion,
			watcher:         w,
		}
		return true, w, nil
	})
	return c
}

func (c *fakeCluster) setList(resourceVersion string, names ...string) {
	list := &unstructured.UnstructuredList{}
	list.SetResourceVersion(resourceVersion)
	for _, name := range names {
		obj := unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("ConfigMap")
		obj.SetNamespace("default")
		obj.SetName(name)
		obj.SetUID(types.UID(name))
		list.Items = append(list.Items, obj)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list = list
}

func (c *fakeCluster) failNextLists(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failLists = n
}

func (c *fakeCluster) nextWatch(t *testing.T) startedWatch {
	t.Helper()
	select {
	case w := <-c.watches:
		return w
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for a watch")
		return startedWatch{}
	}
}

func newTestController(t *testing.T, cluster *fakeCluster) *controller {
	t.Helper()
	c := &controller{
		client: dclient.NewFakeClientWithDisco(cluster, nil, nil),
		restartQueue: workqueue.NewTypedRateLimitingQueue(
			workqueue.NewTypedItemExponentialFailureRateLimiter[schema.GroupVersionResource](10*time.Millisecond, 100*time.Millisecond),
		),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		c.restartQueue.ShutDown()
		c.stopDynamicWatchers()
	})
	c.lock.Lock()
	w, err := c.startWatcher(ctx, logr.Discard(), configMapsGVR, configMapGVK)
	assert.NilError(t, err)
	c.dynamicWatchers[configMapsGVR] = w
	c.lock.Unlock()
	go c.processRestarts(ctx)
	return c
}

func expireWatch(w startedWatch) {
	w.watcher.Error(&apierrors.NewResourceExpired("too old resource version: " + w.resourceVersion).ErrStatus)
}

func assertCached(t *testing.T, c *controller, uid string, want bool) {
	t.Helper()
	_, _, _, cached := c.GetResourceHash(types.UID(uid))
	assert.Equal(t, cached, want, "resource %s", uid)
}

func TestWatcherRestartsFromFreshListOnExpiredResourceVersion(t *testing.T) {
	cluster := newFakeCluster()
	cluster.setList("100", "a", "b")
	c := newTestController(t, cluster)

	var mu sync.Mutex
	var deleted []types.UID
	c.AddEventHandler(func(eventType EventType, uid types.UID, _ schema.GroupVersionKind, _ Resource) {
		if eventType == Deleted {
			mu.Lock()
			defer mu.Unlock()
			deleted = append(deleted, uid)
		}
	})

	first := cluster.nextWatch(t)
	assert.Equal(t, first.resourceVersion, "100")

	// a is deleted and c is created while nothing is watching
	cluster.setList("200", "b", "c")
	expireWatch(first)

	second := cluster.nextWatch(t)
	assert.Equal(t, second.resourceVersion, "200")
	assertCached(t, c, "a", false)
	assertCached(t, c, "b", true)
	assertCached(t, c, "c", true)
	mu.Lock()
	defer mu.Unlock()
	assert.DeepEqual(t, deleted, []types.UID{"a"})
}

func TestWatcherRestartIsRetriedUntilListSucceeds(t *testing.T) {
	cluster := newFakeCluster()
	cluster.setList("100", "a")
	c := newTestController(t, cluster)

	first := cluster.nextWatch(t)
	cluster.setList("300", "a", "b")
	cluster.failNextLists(3)
	expireWatch(first)

	second := cluster.nextWatch(t)
	assert.Equal(t, second.resourceVersion, "300")
	assertCached(t, c, "b", true)
}
