package health

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
)

const watchStabilityPeriod = time.Second

type listWatcher struct {
	cache.ListerWatcherWithContext
	stream               *stream
	unsupportedWatchList bool
}

func (l *listWatcher) IsWatchListSemanticsUnSupported() bool { return l.unsupportedWatchList }

func (l *listWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	return l.ListWithContext(context.Background(), options)
}

func (l *listWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return l.WatchWithContext(context.Background(), options)
}

func (l *listWatcher) ListWithContext(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
	l.stream.begin()
	// A completed list is not recovery until the subsequent watch is established.
	return l.ListerWatcherWithContext.ListWithContext(ctx, options)
}

func (l *listWatcher) WatchWithContext(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
	attempt := l.stream.begin()
	w, err := l.ListerWatcherWithContext.WatchWithContext(ctx, options)
	if err != nil {
		return nil, err
	}
	initialEvents := options.SendInitialEvents != nil && *options.SendInitialEvents
	var recoveryTimer clock.Timer
	if !initialEvents && ctx.Err() == nil {
		// Do not clear an interruption until the returned watch has remained
		// open briefly. A watch can be accepted successfully and then close
		// immediately; client-go may retry that cycle indefinitely.
		recoveryTimer = l.stream.tracker.clock.NewTimer(watchStabilityPeriod)
	}
	out := &monitoredWatch{upstream: w, result: make(chan watch.Event), done: make(chan struct{}), recoveryTimer: recoveryTimer}
	go out.forward(ctx, l.stream, attempt, initialEvents)
	return out, nil
}

type monitoredWatch struct {
	upstream      watch.Interface
	result        chan watch.Event
	done          chan struct{}
	once          sync.Once
	recoveryTimer clock.Timer
}

func (w *monitoredWatch) Stop() {
	w.once.Do(func() {
		close(w.done)
		w.upstream.Stop()
	})
}

func (w *monitoredWatch) ResultChan() <-chan watch.Event { return w.result }

func (w *monitoredWatch) forward(ctx context.Context, s *stream, attempt uint64, initialEvents bool) {
	defer close(w.result)
	defer w.Stop()
	if w.recoveryTimer != nil {
		defer w.recoveryTimer.Stop()
	}
	defer func() {
		if ctx.Err() == nil {
			s.fail(attempt)
		}
	}()
	failed := false
	var recovery <-chan time.Time
	if w.recoveryTimer != nil {
		recovery = w.recoveryTimer.C()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case <-recovery:
			if ctx.Err() == nil {
				s.recover(attempt)
			}
			recovery = nil
		case event, ok := <-w.upstream.ResultChan():
			if !ok {
				return
			}
			if event.Type == watch.Error {
				failed = true
				s.fail(attempt)
				recovery = nil
				if w.recoveryTimer != nil {
					w.recoveryTimer.Stop()
				}
			} else if !initialEvents && !failed && ctx.Err() == nil {
				s.recover(attempt)
				recovery = nil
			}
			select {
			case <-ctx.Done():
				return
			case <-w.done:
				return
			case w.result <- event:
			}
			if initialEvents && !failed && event.Type == watch.Bookmark && ctx.Err() == nil {
				if obj, err := meta.Accessor(event.Object); err == nil && obj.GetAnnotations()[metav1.InitialEventsAnnotationKey] == "true" {
					s.recover(attempt)
					initialEvents = false
				}
			}
		}
	}
}
