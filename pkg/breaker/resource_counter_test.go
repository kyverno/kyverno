// what this file is for: tests entry insertion and deletion, deduplication, count correctness, composite counter behavior, health propagation
// what this file is not for: actual k8s interactions, real watcher behavior
package breaker

import (
	"testing"

	//watchtools "github.com/kyverno/kyverno/cmd/kyverno/watch"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	k8swatch "k8s.io/apimachinery/pkg/watch"
)

type fakeRetryWatcher struct {
	running bool
	ch      chan k8swatch.Event
}

func (f *fakeRetryWatcher) ResultChan() <-chan k8swatch.Event {
	return f.ch
}

func (f *fakeRetryWatcher) IsRunning() bool {
	return f.running
}

func (f *fakeRetryWatcher) Stop() {}

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
