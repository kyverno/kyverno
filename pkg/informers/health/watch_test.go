package health

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	clocktesting "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
)

func testStream(t *testing.T, tracker *Tracker, parent *cache.ListWatch) *listWatcher {
	t.Helper()
	s := &stream{tracker: tracker, name: t.Name()}
	tracker.mu.Lock()
	tracker.streams = append(tracker.streams, s)
	tracker.mu.Unlock()
	return &listWatcher{ListerWatcherWithContext: parent, stream: s}
}

func receive(t *testing.T, w watch.Interface) watch.Event {
	t.Helper()
	select {
	case event, ok := <-w.ResultChan():
		require.True(t, ok)
		return event
	case <-time.After(time.Second):
		t.Fatal("watch did not forward event")
		return watch.Event{}
	}
}

func closed(t *testing.T, w watch.Interface) {
	t.Helper()
	select {
	case _, ok := <-w.ResultChan():
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("watch did not close")
	}
}

func TestWatchInterruptionAndRecovery(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"error event", "EOF"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			clock := clocktesting.NewFakeClock(time.Now())
			tracker := NewTracker(logr.Discard(), clock)
			upstream := watch.NewRaceFreeFake()
			var watchErr error
			lw := testStream(t, tracker, &cache.ListWatch{
				ListWithContextFunc:  func(context.Context, metav1.ListOptions) (runtime.Object, error) { return &corev1.ConfigMapList{}, nil },
				WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) { return upstream, watchErr },
			})
			w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "1"})
			require.NoError(t, err)
			t.Cleanup(w.Stop)
			clock.Step(time.Hour)
			require.Eventually(t, tracker.Ready, time.Second, time.Millisecond, "quiet watches are healthy")
			if failure == "error event" {
				upstream.Error(&metav1.Status{Status: metav1.StatusFailure})
				require.Equal(t, watch.Error, receive(t, w).Type)
				w.Stop()
			} else {
				upstream.Stop()
			}
			closed(t, w)
			require.True(t, tracker.Ready())
			watchErr = errors.New("disconnected")
			for range 3 {
				clock.Step(9 * time.Second)
				_, err = lw.WatchWithContext(t.Context(), metav1.ListOptions{})
				require.Error(t, err)
				require.True(t, tracker.Ready())
			}
			clock.Step(3 * time.Second)
			require.False(t, tracker.Ready(), "retries must not restart the deadline")
			_, err = lw.ListWithContext(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			require.False(t, tracker.Ready(), "a successful LIST alone is not recovery")
			watchErr = nil
			upstream = watch.NewRaceFreeFake()
			recovered, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "2"})
			require.NoError(t, err)
			t.Cleanup(recovered.Stop)
			upstream.Add(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "updated"}})
			require.Equal(t, "updated", receive(t, recovered).Object.(*corev1.ConfigMap).Name)
			require.True(t, tracker.Ready())
		})
	}
}

func TestWatchListRequiresInitialEnd(t *testing.T) {
	t.Parallel()
	clock := clocktesting.NewFakeClock(time.Now())
	tracker := NewTracker(logr.Discard(), clock)
	upstream := watch.NewRaceFreeFake()
	lw := testStream(t, tracker, &cache.ListWatch{WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) { return upstream, nil }})
	w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{SendInitialEvents: ptr.To(true)})
	require.NoError(t, err)
	t.Cleanup(w.Stop)
	clock.Step(GracePeriod)
	require.False(t, tracker.Ready(), "opening a watch-list is not a completed snapshot")
	upstream.Add(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "initial"}})
	receive(t, w)
	upstream.Action(watch.Bookmark, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}})
	receive(t, w)
	require.False(t, tracker.Ready(), "ordinary bookmarks do not finish watch-list initialization")
	upstream.Stop()
	closed(t, w)
	upstream = watch.NewRaceFreeFake()
	w, err = lw.WatchWithContext(t.Context(), metav1.ListOptions{SendInitialEvents: ptr.To(true)})
	require.NoError(t, err)
	t.Cleanup(w.Stop)
	require.False(t, tracker.Ready(), "an incomplete snapshot must preserve the deadline")
	upstream.Action(watch.Bookmark, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "3", Annotations: map[string]string{metav1.InitialEventsAnnotationKey: "true"}}})
	receive(t, w)
	require.Eventually(t, tracker.Ready, time.Second, time.Millisecond)
}

func TestFailedListAndHangingWatch(t *testing.T) {
	t.Parallel()
	clock := clocktesting.NewFakeClock(time.Now())
	tracker := NewTracker(logr.Discard(), clock)
	entered := make(chan struct{})
	lw := testStream(t, tracker, &cache.ListWatch{
		ListWithContextFunc: func(context.Context, metav1.ListOptions) (runtime.Object, error) {
			return nil, errors.New("list failed")
		},
		WatchFuncWithContext: func(ctx context.Context, _ metav1.ListOptions) (watch.Interface, error) {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	_, err := lw.ListWithContext(t.Context(), metav1.ListOptions{})
	require.Error(t, err)
	clock.Step(20 * time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); _, _ = lw.WatchWithContext(ctx, metav1.ListOptions{}) }()
	<-entered
	clock.Step(10 * time.Second)
	require.False(t, tracker.Ready(), "a hanging establishment must not hide an interruption")
	cancel()
	<-done
}

func TestIndependentStreamsAndObsoleteCallbacks(t *testing.T) {
	t.Parallel()
	clock := clocktesting.NewFakeClock(time.Now())
	tracker := NewTracker(logr.Discard(), clock)
	lw := testStream(t, tracker, &cache.ListWatch{WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) {
		return watch.NewRaceFreeFake(), nil
	}})
	other := testStream(t, tracker, &cache.ListWatch{WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) { return nil, errors.New("down") }})
	old, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "1"})
	require.NoError(t, err)
	t.Cleanup(old.Stop)
	_, err = other.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.Error(t, err)
	clock.Step(GracePeriod)
	current, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "2"})
	require.NoError(t, err)
	t.Cleanup(current.Stop)
	clock.Step(watchStabilityPeriod)
	require.Eventually(t, func() bool {
		tracker.mu.RLock()
		defer tracker.mu.RUnlock()
		return !tracker.streams[0].interrupted
	}, time.Second, time.Millisecond)
	old.Stop()
	closed(t, old)
	require.False(t, tracker.Ready(), "one recovered stream must not hide another")
	other.stream.stop()
	require.True(t, tracker.Ready(), "the old watch must not invalidate its replacement")
}

func TestTransientInterruption(t *testing.T) {
	t.Parallel()
	clock := clocktesting.NewFakeClock(time.Now())
	tracker := NewTracker(logr.Discard(), clock)
	lw := testStream(t, tracker, &cache.ListWatch{WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) {
		return watch.NewRaceFreeFake(), nil
	}})
	w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "1"})
	require.NoError(t, err)
	w.Stop()
	closed(t, w)
	clock.Step(GracePeriod - time.Nanosecond)
	require.True(t, tracker.Ready())
	w, err = lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "1"})
	require.NoError(t, err)
	t.Cleanup(w.Stop)
	clock.Step(time.Hour)
	require.Eventually(t, tracker.Ready, time.Second, time.Millisecond)
}

func TestRepeatedSuccessfulWatchClosuresPreserveDeadline(t *testing.T) {
	t.Parallel()
	clock := clocktesting.NewFakeClock(time.Now())
	tracker := NewTracker(logr.Discard(), clock)
	var upstream watch.Interface
	lw := testStream(t, tracker, &cache.ListWatch{WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) {
		upstream = watch.NewRaceFreeFake()
		return upstream, nil
	}})

	for range int(GracePeriod / watchStabilityPeriod) {
		w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "1"})
		require.NoError(t, err)
		upstream.Stop()
		closed(t, w)
		clock.Step(watchStabilityPeriod - time.Nanosecond)
	}
	w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{ResourceVersion: "1"})
	require.NoError(t, err)
	t.Cleanup(w.Stop)

	// The API server accepted the latest watch, but every connection has
	// immediately closed. Reopening a watch must not clear the original
	// interruption deadline until a connection has remained stable.
	clock.Step(30 * time.Nanosecond)
	require.False(t, tracker.Ready())
}

func TestCancellationWhileForwarding(t *testing.T) {
	t.Parallel()
	clock := clocktesting.NewFakeClock(time.Now())
	tracker := NewTracker(logr.Discard(), clock)
	upstream := watch.NewRaceFreeFake()
	lw := testStream(t, tracker, &cache.ListWatch{WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) { return upstream, nil }})
	ctx, cancel := context.WithCancel(t.Context())
	w, err := lw.WatchWithContext(ctx, metav1.ListOptions{ResourceVersion: "1"})
	require.NoError(t, err)
	upstream.Add(&corev1.ConfigMap{})
	receive(t, w)
	cancel()
	// Stop is safe concurrently and does not depend on draining ResultChan.
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(w.Stop)
	}
	wg.Wait()
	for range w.ResultChan() {
	}
	require.True(t, upstream.IsStopped())
	clock.Step(time.Hour)
	require.True(t, tracker.Ready(), "intentional cancellation does not create a failure")
}

func TestWatchListCapabilityPreserved(t *testing.T) {
	t.Parallel()
	for _, unsupported := range []bool{false, true} {
		t.Run(fmt.Sprint(unsupported), func(t *testing.T) {
			t.Parallel()
			tracker := NewTracker(logr.Discard(), clocktesting.NewFakeClock(time.Now()))
			options := make(chan metav1.ListOptions, 1)
			parent := &capabilityListWatch{unsupported: unsupported, ListWatch: &cache.ListWatch{
				ListWithContextFunc: func(context.Context, metav1.ListOptions) (runtime.Object, error) {
					return &corev1.ConfigMapList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}, nil
				},
				WatchFuncWithContext: func(_ context.Context, opts metav1.ListOptions) (watch.Interface, error) {
					options <- opts
					return watch.NewRaceFreeFake(), nil
				},
			}}
			inf := tracker.NewInformer(parent, &corev1.ConfigMap{}, 0, nil)
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			go func() { defer close(done); inf.RunWithContext(ctx) }()
			defer func() { cancel(); <-done }()
			select {
			case opts := <-options:
				require.Equal(t, !unsupported, opts.SendInitialEvents != nil && *opts.SendInitialEvents)
			case <-time.After(time.Second):
				t.Fatal("informer did not establish its watch")
			}
		})
	}
}

type capabilityListWatch struct {
	*cache.ListWatch
	unsupported bool
}

func (l *capabilityListWatch) IsWatchListSemanticsUnSupported() bool { return l.unsupported }
