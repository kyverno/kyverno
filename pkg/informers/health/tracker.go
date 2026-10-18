// Package health tracks interruptions of the list/watch streams feeding admission
// informers. It measures connectivity, not completion of downstream reconciliation.
package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
)

const GracePeriod = 30 * time.Second

// Tracker keeps independent health state for each informer it constructs.
// Its zero value is not usable; construct it with NewTracker.
type Tracker struct {
	mu      sync.RWMutex
	streams []*stream
	clock   clock.Clock
	logger  logr.Logger
}

type stream struct {
	tracker       *Tracker
	name          string
	attempt       uint64
	interruptedAt time.Time
	interrupted   bool
	stopped       bool
}

func NewTracker(logger logr.Logger, clock clock.Clock) *Tracker {
	return &Tracker{logger: logger, clock: clock}
}

// Ready tolerates short interruptions. Idle streams need no periodic events.
func (t *Tracker) Ready() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, s := range t.streams {
		if !s.stopped && s.interrupted && t.clock.Since(s.interruptedAt) >= GracePeriod {
			return false
		}
	}
	return true
}

// NewInformer has the same signature as cache.NewSharedIndexInformer, including
// controller-runtime's NewInformer hook. Each call registers a distinct stream.
func (t *Tracker) NewInformer(lw cache.ListerWatcher, obj runtime.Object, resync time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	t.mu.Lock()
	s := &stream{tracker: t, name: fmt.Sprintf("%T/%d", obj, len(t.streams))}
	t.streams = append(t.streams, s)
	t.mu.Unlock()
	monitored := &listWatcher{ListerWatcherWithContext: cache.ToListerWatcherWithContext(lw), stream: s}
	if semantics, ok := lw.(interface{ IsWatchListSemanticsUnSupported() bool }); ok {
		monitored.unsupportedWatchList = semantics.IsWatchListSemanticsUnSupported()
	}
	return &informer{SharedIndexInformer: cache.NewSharedIndexInformer(monitored, obj, resync, indexers), stream: s}
}

// begin invalidates callbacks from older requests and starts the initial
// synchronization deadline. It never extends an existing interruption.
func (s *stream) begin() uint64 {
	t := s.tracker
	t.mu.Lock()
	defer t.mu.Unlock()
	s.attempt++
	s.interruptLocked()
	return s.attempt
}

func (s *stream) interruptLocked() {
	if !s.stopped && !s.interrupted {
		s.interrupted = true
		s.interruptedAt = s.tracker.clock.Now()
		s.tracker.logger.V(2).Info("admission informer stream interrupted", "stream", s.name)
	}
}

func (s *stream) fail(attempt uint64) {
	t := s.tracker
	t.mu.Lock()
	defer t.mu.Unlock()
	if s.attempt == attempt {
		s.interruptLocked()
	}
}

func (s *stream) recover(attempt uint64) {
	t := s.tracker
	t.mu.Lock()
	defer t.mu.Unlock()
	if !s.stopped && s.attempt == attempt && s.interrupted {
		s.interrupted = false
		t.logger.V(2).Info("admission informer stream recovered", "stream", s.name)
	}
}

func (s *stream) stop() {
	t := s.tracker
	t.mu.Lock()
	defer t.mu.Unlock()
	s.stopped = true
}

type informer struct {
	cache.SharedIndexInformer
	stream *stream
}

func (i *informer) Run(stopCh <-chan struct{}) {
	defer i.stream.stop()
	i.SharedIndexInformer.Run(stopCh)
}

func (i *informer) RunWithContext(ctx context.Context) {
	defer i.stream.stop()
	i.SharedIndexInformer.RunWithContext(ctx)
}
