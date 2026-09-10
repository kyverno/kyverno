// what this file is for: tests entry insertion and deletion, deduplication, count correctness, composite counter behavior, health propagation
// what this file is not for: actual k8s interactions, real watcher behavior
package breaker

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	//watchtools "github.com/kyverno/kyverno/cmd/kyverno/watch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	k8swatch "k8s.io/apimachinery/pkg/watch"
	metadatafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
)

type fakeRetryWatcher struct {
	running bool
	stopped bool
	ch      chan k8swatch.Event
}

func (f *fakeRetryWatcher) ResultChan() <-chan k8swatch.Event {
	return f.ch
}

func (f *fakeRetryWatcher) IsRunning() bool {
	return f.running
}

func (f *fakeRetryWatcher) Stop() {
	f.stopped = true
}

// counter tests

func TestCounter_RecordAndForget(t *testing.T) {
	fw := &fakeRetryWatcher{
		running: true,
		ch:      make(chan k8swatch.Event),
	}

	c := &counter{
		entries:      sets.New[types.UID](),
		retryWatcher: fw,
	}

	c.Record("uid-1")
	c.Record("uid-2")
	c.Record("uid-2") // dedup

	count, running := c.Count()
	if count != 2 {
		t.Fatalf("expected count=2, got %d", count)
	}
	if !running {
		t.Fatalf("expected watcher to be running")
	}

	c.Forget("uid-1")
	count, _ = c.Count()
	if count != 1 {
		t.Fatalf("expected count=1 after forget, got %d", count)
	}
}

func TestCounter_ForgetNonExistingUID(t *testing.T) {
	c := &counter{
		entries: sets.New[types.UID](),
		retryWatcher: &fakeRetryWatcher{
			running: true,
			ch:      make(chan k8swatch.Event),
		},
	}

	// Should not panic
	c.Forget("does-not-exist")

	count, _ := c.Count()
	if count != 0 {
		t.Fatalf("expected count=0, got %d", count)
	}
}

func TestCounter_NotRunning(t *testing.T) {
	c := &counter{
		entries: sets.New[types.UID]("uid-1"),
		retryWatcher: &fakeRetryWatcher{
			running: false,
			ch:      make(chan k8swatch.Event),
		},
	}

	count, running := c.Count()
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}
	if running {
		t.Fatalf("expected watcher to NOT be running")
	}
}

func TestCounter_Stop(t *testing.T) {
	fw := &fakeRetryWatcher{
		running: true,
		ch:      make(chan k8swatch.Event),
	}

	c := &counter{
		entries:      sets.New[types.UID](),
		retryWatcher: fw,
	}

	c.Stop()
	if !fw.stopped {
		t.Fatalf("expected underlying retry watcher to be stopped")
	}
}

func TestStartAdmissionReportsCounter_PartialInitFailure(t *testing.T) {
	client := metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme())
	// first counter starts fine
	client.PrependReactor("list", "ephemeralreports", func(action k8stesting.Action) (bool, kruntime.Object, error) {
		list := &metav1.List{}
		list.SetResourceVersion("1")
		return true, list, nil
	})
	// second counter fails to start
	client.PrependReactor("list", "clusterephemeralreports", func(action k8stesting.Action) (bool, kruntime.Object, error) {
		return true, nil, errors.New("the server is currently unable to handle the request")
	})

	before := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		if _, err := StartAdmissionReportsCounter(context.TODO(), client); err == nil {
			t.Fatal("expected an error")
		}
	}
	// the successfully started ephemeralreports watchers must be stopped,
	// otherwise each failed call above leaks their goroutines
	deadline := time.Now().Add(5 * time.Second)
	for {
		if runtime.NumGoroutine() <= before+3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("goroutines leaked: before=%d now=%d", before, runtime.NumGoroutine())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// composite counter tests

type fakeCounter struct {
	count   int
	running bool
}

func (f fakeCounter) Count() (int, bool) {
	return f.count, f.running
}

func TestCompositeCounter_AllRunning(t *testing.T) {
	c := composite{
		inner: []Counter{
			fakeCounter{count: 2, running: true},
			fakeCounter{count: 3, running: true},
		},
	}

	count, running := c.Count()
	if count != 5 {
		t.Fatalf("expected count=5, got %d", count)
	}
	if !running {
		t.Fatalf("expected composite to be running")
	}
}

func TestCompositeCounter_OneNotRunning(t *testing.T) {
	c := composite{
		inner: []Counter{
			fakeCounter{count: 2, running: true},
			fakeCounter{count: 3, running: false},
		},
	}

	count, running := c.Count()
	if count != 0 {
		t.Fatalf("expected count=0 when one counter is down, got %d", count)
	}
	if running {
		t.Fatalf("expected composite to NOT be running")
	}
}
