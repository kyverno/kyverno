package watch

import (
	"io"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8swatch "k8s.io/apimachinery/pkg/watch"
)

// fakeObj implements runtime.Object and resourceVersionGetter.
type fakeObj struct {
	rv string
}

// Satisfy runtime.Object.
func (f *fakeObj) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (f *fakeObj) DeepCopyObject() runtime.Object   { return &fakeObj{rv: f.rv} }

// resourceVersionGetter method used by the watcher under test.
func (f *fakeObj) GetResourceVersion() string { return f.rv }

// fakeWatcherClient implements a minimal Watcher returning a sequence of watchers/errors.
type fakeWatcherClient struct {
	watchers []k8swatch.Interface
	errs     []error
	calls    int
	opts     []metav1.ListOptions
}

func (f *fakeWatcherClient) Watch(options metav1.ListOptions) (k8swatch.Interface, error) {
	f.opts = append(f.opts, options)
	idx := f.calls
	f.calls++
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.watchers) {
		return f.watchers[idx], nil
	}
	return k8swatch.NewRaceFreeFake(), nil
}

func TestNewRetryWatcherRejectsEmptyOrZeroRV(t *testing.T) {
	fc := &fakeWatcherClient{}
	if _, err := NewRetryWatcher("", fc); err == nil {
		t.Fatalf("expected error for empty initial RV, got nil")
	}
	if _, err := NewRetryWatcher("0", fc); err == nil {
		t.Fatalf("expected error for '0' initial RV, got nil")
	}
}

func TestRetryWatcherProcessesEventsAndUpdatesResourceVersion(t *testing.T) {
	fw := k8swatch.NewRaceFreeFake()
	fw.Add(&fakeObj{rv: "10"})
	fw.Modify(&fakeObj{rv: "11"})
	fw.Delete(&fakeObj{rv: "12"})
	fw.Action(k8swatch.Bookmark, &fakeObj{rv: "13"})

	rw, err := newRetryWatcher("9", &fakeWatcherClient{watchers: []k8swatch.Interface{fw}}, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("newRetryWatcher error: %v", err)
	}
	defer rw.Stop()

	got := make([]k8swatch.Event, 0, 3)
	timeout := time.After(2 * time.Second)
	for len(got) < 3 {
		select {
		case ev, ok := <-rw.ResultChan():
			if !ok {
				t.Fatalf("result channel closed prematurely")
			}
			got = append(got, ev)
		case <-timeout:
			t.Fatalf("timed out waiting for events, got %d", len(got))
		}
	}

	if got[0].Type != k8swatch.Added || got[1].Type != k8swatch.Modified || got[2].Type != k8swatch.Deleted {
		t.Fatalf("unexpected event types: %+v", got)
	}

	// Small delay to ensure Bookmark event is processed
	time.Sleep(100 * time.Millisecond)

	// Thread-safe read of lastResourceVersion
	rw.mu.RLock()
	actualRV := rw.lastResourceVersion
	rw.mu.RUnlock()

	if actualRV != "13" {
		t.Fatalf("expected lastResourceVersion=13, got=%s", actualRV)
	}
}

func TestRetryWatcherStopsOnGoneError(t *testing.T) {
	fw := k8swatch.NewRaceFreeFake()
	fw.Error(&metav1.Status{Code: http.StatusGone})
	rw, err := newRetryWatcher("100", &fakeWatcherClient{watchers: []k8swatch.Interface{fw}}, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("newRetryWatcher error: %v", err)
	}
	defer rw.Stop()

	select {
	case ev := <-rw.ResultChan():
		if ev.Type != k8swatch.Error {
			t.Fatalf("expected Error event, got %v", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for error event")
	}

	select {
	case <-rw.Done():
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for watcher to stop after 410 Gone")
	}
}

func TestRetryWatcherRetriesOnEOFThenSucceeds(t *testing.T) {
	fw := k8swatch.NewRaceFreeFake()
	fw.Add(&fakeObj{rv: "21"})
	// The first call to Watch() will return an error.
	// The second call will return the watcher `fw`.
	fc := &fakeWatcherClient{
		errs:     []error{io.EOF},
		watchers: []k8swatch.Interface{nil, fw},
	}

	rw, err := newRetryWatcher("20", fc, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("newRetryWatcher error: %v", err)
	}
	defer rw.Stop()

	select {
	case ev := <-rw.ResultChan():
		if ev.Type != k8swatch.Added {
			t.Fatalf("expected Added event after retry, got %v", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for event after EOF retry")
	}

	if len(fc.opts) < 2 || fc.opts[0].ResourceVersion != "20" {
		t.Fatalf("expected first watch to use RV=20, got opts=%+v", fc.opts)
	}
}

type noRVObj struct{ metav1.TypeMeta }

// Satisfy runtime.Object for noRVObj.
func (o *noRVObj) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (o *noRVObj) DeepCopyObject() runtime.Object   { return &noRVObj{} }

func TestRetryWatcherErrorOnNonRVObjectStops(t *testing.T) {
	funcObj := &noRVObj{}

	fw := k8swatch.NewRaceFreeFake()
	fw.Add(funcObj)

	rw, err := newRetryWatcher("50", &fakeWatcherClient{watchers: []k8swatch.Interface{fw}}, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("newRetryWatcher error: %v", err)
	}
	defer rw.Stop()

	select {
	case ev := <-rw.ResultChan():
		if ev.Type != k8swatch.Error {
			t.Fatalf("expected Error event for non-RV object, got %v", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for error event")
	}

	select {
	case <-rw.Done():
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for watcher to stop after non-RV object")
	}
}

func TestRetryWatcherStop(t *testing.T) {
	rw, err := newRetryWatcher("77", &fakeWatcherClient{watchers: []k8swatch.Interface{k8swatch.NewRaceFreeFake()}}, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("newRetryWatcher error: %v", err)
	}
	rw.Stop()
	select {
	case <-rw.Done():
	case <-time.After(1 * time.Second):
		t.Fatalf("expected Done to be closed after Stop()")
	}
}
