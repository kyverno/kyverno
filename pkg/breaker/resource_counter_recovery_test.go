package breaker

import (
	"context"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	k8swatch "k8s.io/apimachinery/pkg/watch"
	metadatafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
)

var testGVR = schema.GroupVersionResource{Group: "reports.kyverno.io", Version: "v1", Resource: "ephemeralreports"}

// listResult is one canned response for the List call, in order.
type listResult struct {
	resourceVersion string
	items           []metav1.PartialObjectMetadata
}

// fakeReportsClient serves a scripted sequence of Lists and hands out a fresh
// watcher for every Watch call. Receive on watchers to get the nth one, in order.
type fakeReportsClient struct {
	*metadatafake.FakeMetadataClient

	mu        sync.Mutex
	lists     []listResult
	listed    int
	expiredRV string
	watchers  chan *k8swatch.RaceFreeFakeWatcher
}

// expireRV makes every Watch call starting at rv fail with 410, however often
// it is retried.
func (c *fakeReportsClient) expireRV(rv string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expiredRV = rv
}

func newFakeReportsClient(lists ...listResult) *fakeReportsClient {
	scheme := metadatafake.NewTestScheme()
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		panic(err)
	}
	c := &fakeReportsClient{
		FakeMetadataClient: metadatafake.NewSimpleMetadataClient(scheme),
		lists:              lists,
		watchers:           make(chan *k8swatch.RaceFreeFakeWatcher, 8),
	}
	c.PrependReactor("list", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		result := c.lists[min(c.listed, len(c.lists)-1)]
		c.listed++
		list := &metav1.List{ListMeta: metav1.ListMeta{ResourceVersion: result.resourceVersion}}
		for i := range result.items {
			list.Items = append(list.Items, runtime.RawExtension{Object: &result.items[i]})
		}
		return true, list, nil
	})
	c.PrependWatchReactor("*", func(action k8stesting.Action) (bool, k8swatch.Interface, error) {
		rv := action.(k8stesting.WatchAction).GetWatchRestrictions().ResourceVersion
		c.mu.Lock()
		expired := c.expiredRV != "" && rv == c.expiredRV
		c.mu.Unlock()
		if expired {
			return true, nil, apierrors.NewResourceExpired("too old resource version: " + rv)
		}
		w := k8swatch.NewRaceFreeFake()
		select {
		case c.watchers <- w:
		default: // nobody is watching for more watchers, don't block the client
		}
		return true, w, nil
	})
	return c
}

func report(name string, uid types.UID, source string) metav1.PartialObjectMetadata {
	return metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			UID:    uid,
			Labels: map[string]string{"audit.kyverno.io/source": source},
		},
	}
}

func admissionOnly(lo *metav1.ListOptions) {
	lo.LabelSelector = "audit.kyverno.io/source==admission"
}

// awaitHealthyCount waits for the counter to report itself running with the given count.
func awaitHealthyCount(t *testing.T, c Counter, want int) {
	t.Helper()
	var count int
	var running bool
	if err := wait.PollUntilContextTimeout(context.Background(), 10*time.Millisecond, 15*time.Second, true, func(context.Context) (bool, error) {
		count, running = c.Count()
		return running && count == want, nil
	}); err != nil {
		t.Fatalf("Count() = (%d, %v), want (%d, true)", count, running, want)
	}
}

// A watch that dies on "resource version too old" must not leave the counter
// unhealthy for good: that keeps the reports breaker open and silently drops
// every admission report for the lifetime of the process.
func TestResourceCounterRecoversFromResourceVersionTooOld(t *testing.T) {
	client := newFakeReportsClient(
		listResult{resourceVersion: "100", items: []metav1.PartialObjectMetadata{
			report("a", "uid-a", "admission"),
			report("b", "uid-b", "admission"),
		}},
		listResult{resourceVersion: "200", items: []metav1.PartialObjectMetadata{
			report("c", "uid-c", "admission"),
		}},
	)

	c, err := StartResourceCounter(t.Context(), client, testGVR, admissionOnly)
	if err != nil {
		t.Fatalf("StartResourceCounter: %v", err)
	}
	awaitHealthyCount(t, c, 2)

	(<-client.watchers).Error(&metav1.Status{
		Status: metav1.StatusFailure,
		Code:   410,
		Reason: metav1.StatusReasonGone,
	})

	// Back to healthy, and holding the entries of the second List - which it can
	// only have if it re-Listed for a fresh resource version.
	awaitHealthyCount(t, c, 1)
}

// The apiserver usually rejects a stale resource version when the watch is
// established, not mid-stream. Retrying that can never succeed, so the counter
// has to relist here too.
func TestResourceCounterRecoversFromRejectedWatch(t *testing.T) {
	client := newFakeReportsClient(
		listResult{resourceVersion: "100", items: []metav1.PartialObjectMetadata{
			report("a", "uid-a", "admission"),
			report("b", "uid-b", "admission"),
		}},
		listResult{resourceVersion: "200", items: []metav1.PartialObjectMetadata{
			report("c", "uid-c", "admission"),
		}},
	)

	c, err := StartResourceCounter(t.Context(), client, testGVR, admissionOnly)
	if err != nil {
		t.Fatalf("StartResourceCounter: %v", err)
	}
	awaitHealthyCount(t, c, 2)

	// The watch drops, and every reconnect at resource version 100 is rejected
	// because it has been compacted away.
	client.expireRV("100")
	(<-client.watchers).Stop()

	awaitHealthyCount(t, c, 1)
}

// The seeding List has to be restricted the same way the watch is, or the
// admission counter counts background scan reports and the other way around.
func TestResourceCounterSeedsOnlyMatchingResources(t *testing.T) {
	client := newFakeReportsClient(
		listResult{resourceVersion: "100", items: []metav1.PartialObjectMetadata{
			report("a", "uid-a", "admission"),
			report("b", "uid-b", "background-scan"),
		}},
	)

	c, err := StartResourceCounter(t.Context(), client, testGVR, admissionOnly)
	if err != nil {
		t.Fatalf("StartResourceCounter: %v", err)
	}

	if count, _ := c.Count(); count != 1 {
		t.Fatalf("Count() = %d, want 1: only the admission report should be counted", count)
	}
}
