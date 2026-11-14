package datacache

import (
	"context"
	"errors"
	"maps"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type fetcher struct {
	calls  atomic.Int32
	digest string
	err    error
	delay  time.Duration
}

func (f *fetcher) fetch(ctx context.Context) (map[string]any, error) {
	f.calls.Add(1)
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if f.err != nil {
		return nil, f.err
	}
	return map[string]any{"digest": f.digest}, nil
}

type recorder struct {
	mu sync.Mutex
	n  map[string]int
}

func (r *recorder) RecordRequest(_ context.Context, result metrics.ImageDataCacheResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == nil {
		r.n = map[string]int{}
	}
	r.n[string(result)]++
}

func (r *recorder) counts() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return maps.Clone(r.n)
}

// metricsManager overrides the one accessor the cache uses; any other call panics.
type metricsManager struct {
	metrics.MetricsConfigManager
	rec *recorder
}

func (m *metricsManager) ImageDataCacheMetrics() metrics.ImageDataCacheMetrics { return m.rec }

func newCache(t *testing.T, opts ...Option) Client {
	t.Helper()
	c, err := New(opts...)
	require.NoError(t, err)
	return c
}

// fetchStarted waits until the first fetch is under way.
func fetchStarted(t *testing.T, f *fetcher) {
	t.Helper()
	require.Eventually(t, func() bool { return f.calls.Load() >= 1 }, 5*time.Second, time.Millisecond)
}

func TestTagHitFetchesOnce(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTagTTLDuration(time.Minute))
	f := &fetcher{digest: digestA}
	for range 3 {
		data, err := c.Get(t.Context(), "nginx:1.25", f.fetch)
		require.NoError(t, err)
		assert.Equal(t, digestA, data["digest"])
	}
	assert.EqualValues(t, 1, f.calls.Load())
}

func TestZeroTTLDisablesCaching(t *testing.T) {
	t.Parallel()
	c := newCache(t) // both TTLs are 0
	f := &fetcher{digest: digestA}
	for _, ref := range []string{"nginx:1.25", "nginx@" + digestA} {
		for range 2 {
			_, err := c.Get(t.Context(), ref, f.fetch)
			require.NoError(t, err)
		}
	}
	assert.EqualValues(t, 4, f.calls.Load())
}

func TestErrorsAreNotCached(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTagTTLDuration(time.Minute))
	f := &fetcher{err: errors.New("registry down")}
	for range 2 {
		_, err := c.Get(t.Context(), "nginx:1.25", f.fetch)
		require.Error(t, err)
	}
	assert.EqualValues(t, 2, f.calls.Load())
	f.err = nil
	f.digest = digestA
	_, err := c.Get(t.Context(), "nginx:1.25", f.fetch)
	require.NoError(t, err)
}

func TestInvalidReferenceIsLeftToTheFetcher(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTagTTLDuration(time.Minute))
	f := &fetcher{err: errors.New("invalid reference")}
	_, err := c.Get(t.Context(), "not a valid ref!!", f.fetch)
	require.Error(t, err)
	assert.EqualValues(t, 1, f.calls.Load())
}

func TestDigestEntryOutlivesTagTTL(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTTLDuration(time.Minute), WithTagTTLDuration(50*time.Millisecond))
	f := &fetcher{digest: digestA}
	for _, ref := range []string{"nginx:1.25", "nginx@" + digestA} {
		_, err := c.Get(t.Context(), ref, f.fetch)
		require.NoError(t, err)
	}
	time.Sleep(120 * time.Millisecond)
	for _, ref := range []string{"nginx:1.25", "nginx@" + digestA} {
		_, err := c.Get(t.Context(), ref, f.fetch)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 3, f.calls.Load(), "only the tag entry expired")
}

func TestEachReferenceKeepsItsOwnFields(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTagTTLDuration(time.Minute))
	fetch := func(image, tag string) func(context.Context) (map[string]any, error) {
		return func(context.Context) (map[string]any, error) {
			return map[string]any{"image": image, "tag": tag, "digest": digestA}, nil
		}
	}
	_, err := c.Get(t.Context(), "nginx:1.27", fetch("nginx:1.27", "1.27"))
	require.NoError(t, err)
	other, err := c.Get(t.Context(), "nginx:latest", fetch("nginx:latest", "latest"))
	require.NoError(t, err)
	assert.Equal(t, "latest", other["tag"])
	again, err := c.Get(t.Context(), "nginx:1.27", nil)
	require.NoError(t, err)
	assert.Equal(t, "1.27", again["tag"])
}

func TestWaitingCallerLeavesOnItsOwnContext(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTagTTLDuration(time.Minute))
	f := &fetcher{digest: digestA, delay: 300 * time.Millisecond}
	done := make(chan struct{})
	go func() { defer close(done); _, _ = c.Get(t.Context(), "nginx:1.25", f.fetch) }()
	defer func() { <-done }() // a leftover Get would race with tests that swap the metrics manager
	fetchStarted(t, f)
	waiter, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Get(waiter, "nginx:1.25", f.fetch)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 200*time.Millisecond)
	assert.EqualValues(t, 1, f.calls.Load())
}

func TestPanicInFetchIsReturnedAsAnError(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTagTTLDuration(time.Minute))
	_, err := c.Get(t.Context(), "nginx:1.25", func(context.Context) (map[string]any, error) { panic("boom") })
	require.ErrorContains(t, err, "boom")
}

func TestWaiterGetsDataWhenTheFirstCallerTimesOut(t *testing.T) {
	t.Parallel()
	c := newCache(t, WithTagTTLDuration(time.Minute))
	f := &fetcher{digest: digestA, delay: 300 * time.Millisecond}
	first, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	firstErr := make(chan error, 1)
	go func() { _, err := c.Get(first, "nginx:1.25", f.fetch); firstErr <- err }()
	fetchStarted(t, f)
	data, err := c.Get(t.Context(), "nginx:1.25", f.fetch)
	require.NoError(t, err)
	assert.Equal(t, digestA, data["digest"])
	require.ErrorIs(t, <-firstErr, context.DeadlineExceeded)
	_, err = c.Get(t.Context(), "nginx:1.25", f.fetch)
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.calls.Load(), "one fetch served both callers and was cached")
}

func TestWaitersSharingAFetchAreCountedAsHits(t *testing.T) {
	// not parallel: the metrics manager is process-wide
	rec := &recorder{}
	metrics.SetManager(&metricsManager{rec: rec})
	t.Cleanup(func() { metrics.SetManager(nil) })
	c := newCache(t, WithTagTTLDuration(time.Minute))
	release := make(chan struct{})
	f := &fetcher{}
	fetch := func(context.Context) (map[string]any, error) {
		f.calls.Add(1)
		<-release
		return map[string]any{"digest": digestA}, nil
	}
	var started, wg sync.WaitGroup
	for range 5 {
		started.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			started.Done()
			_, err := c.Get(t.Context(), "nginx:1.25", fetch)
			assert.NoError(t, err)
		}()
	}
	started.Wait()
	fetchStarted(t, f)
	// singleflight waiters can't be observed; this only gives them time to join.
	// A caller that joins late gets a cache hit instead, so it cannot fail the test.
	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, 1, f.calls.Load(), "callers blocked on a fetch don't start their own")
	close(release)
	wg.Wait()
	assert.EqualValues(t, 1, f.calls.Load(), "concurrent misses share one fetch")
	assert.Equal(t, map[string]int{"miss": 1, "hit": 4}, rec.counts())
}
