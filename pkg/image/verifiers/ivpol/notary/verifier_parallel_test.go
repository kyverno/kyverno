package notary

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPool returns a pool of its own, so tests don't share referrerPool.
func testPool(extra int, hedgeDelay time.Duration) *verifyPool {
	return &verifyPool{extra: make(chan struct{}, extra), hedgeDelay: hedgeDelay}
}

// A later referrer that verifies first doesn't win over an earlier one, an
// earlier one that fails is still waited for, and verifications after the
// winner are cancelled.
func Test_firstVerifiedEarliestWins(t *testing.T) {
	t.Parallel()
	started3, cancelled3, returned1 := make(chan struct{}), make(chan struct{}), make(chan struct{})
	found, errs := testPool(3, 0).firstVerified(context.Background(), 4, func(ctx context.Context, i int) error {
		switch i {
		case 0:
			<-returned1
			return errors.New("referrer 0 failed")
		case 1:
			defer close(returned1)
			<-cancelled3
			return nil
		case 2:
			<-started3
			return nil
		default:
			close(started3)
			<-ctx.Done()
			close(cancelled3)
			return ctx.Err()
		}
	})
	assert.Equal(t, 1, found)
	assert.Equal(t, []string{"referrer 0 failed"}, errStrings(errs))
}

// Once a referrer verifies, no later one starts.
func Test_firstVerifiedStopsStarting(t *testing.T) {
	t.Parallel()
	var started [12]atomic.Bool
	found, errs := testPool(3, 0).firstVerified(context.Background(), 12, func(ctx context.Context, i int) error {
		started[i].Store(true)
		if i == 0 {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	})
	assert.Equal(t, 0, found)
	assert.Empty(t, errs)
	// Only 1 to 3 can have started alongside 0.
	for i := 4; i < 12; i++ {
		assert.False(t, started[i].Load(), "referrer %d started", i)
	}
}

// The first referrer runs alone: when it verifies within the hedge delay no
// other starts, and when it is slower others start alongside it.
func Test_firstVerifiedHedgesFirst(t *testing.T) {
	t.Parallel()
	var started [4]atomic.Bool
	found, _ := testPool(3, time.Hour).firstVerified(context.Background(), 4, func(ctx context.Context, i int) error {
		started[i].Store(true)
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	assert.Equal(t, 0, found)
	for i := 1; i < 4; i++ {
		assert.False(t, started[i].Load(), "referrer %d started", i)
	}

	started1 := make(chan struct{})
	found, _ = testPool(3, 10*time.Millisecond).firstVerified(context.Background(), 2, func(ctx context.Context, i int) error {
		if i == 0 {
			<-started1
			return nil
		}
		close(started1)
		return errors.New("referrer 1 failed")
	})
	assert.Equal(t, 0, found)
}

// Callers sharing a pool each run one verification of their own, plus at most
// the pool's extra ones between them.
func Test_firstVerifiedSharesExtra(t *testing.T) {
	t.Parallel()
	const callers, extra = 3, 1
	pool := testPool(extra, 0)
	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			found, errs := pool.firstVerified(context.Background(), 6, func(ctx context.Context, i int) error {
				now := running.Add(1)
				for p := peak.Load(); now > p && !peak.CompareAndSwap(p, now); p = peak.Load() {
				}
				time.Sleep(2 * time.Millisecond)
				running.Add(-1)
				return fmt.Errorf("referrer %d failed", i)
			})
			assert.Equal(t, -1, found)
			assert.Len(t, errs, 6)
		})
	}
	wg.Wait()
	assert.LessOrEqual(t, peak.Load(), int32(callers+extra))
}

// For random outcomes and timings, the result is the same as verifying in
// order until one verifies.
func Test_firstVerifiedMatchesSequential(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec
	for range 200 {
		n := 1 + rng.IntN(10)
		pool := testPool(rng.IntN(4), time.Duration(rng.IntN(2))*time.Millisecond)
		verifies := make([]bool, n)
		delays := make([]time.Duration, n)
		for i := range n {
			verifies[i] = rng.IntN(3) == 0
			delays[i] = time.Duration(rng.IntN(500)) * time.Microsecond
		}
		wantFound, wantErrs := -1, []string{}
		for i := range n {
			if verifies[i] {
				wantFound = i
				break
			}
			wantErrs = append(wantErrs, fmt.Sprintf("referrer %d failed", i))
		}

		found, errs := pool.firstVerified(context.Background(), n, func(ctx context.Context, i int) error {
			select {
			case <-time.After(delays[i]):
			case <-ctx.Done():
				return ctx.Err()
			}
			if verifies[i] {
				return nil
			}
			return fmt.Errorf("referrer %d failed", i)
		})
		assert.Equal(t, wantFound, found, "verifies %v", verifies)
		assert.Equal(t, wantErrs, errStrings(errs), "verifies %v", verifies)
	}
}

// A panic is raised from the caller, with the stack where it happened, only if
// verifying in order would have reached it.
func Test_firstVerifiedPanics(t *testing.T) {
	t.Parallel()
	func() {
		defer func() {
			p, ok := recover().(*verifyPanic)
			require.True(t, ok, "re-raised a *verifyPanic")
			assert.Equal(t, "referrer 1 panicked", p.value)
			// The engine formats the recovered value with %v.
			assert.Contains(t, fmt.Sprintf("%v", p), "Test_firstVerifiedPanics")
		}()
		testPool(3, 0).firstVerified(context.Background(), 4, func(ctx context.Context, i int) error {
			switch i {
			case 1:
				panic("referrer 1 panicked")
			case 2:
				return nil
			default:
				return fmt.Errorf("referrer %d failed", i)
			}
		})
	}()

	panicked := make(chan struct{})
	found, errs := testPool(3, 0).firstVerified(context.Background(), 4, func(ctx context.Context, i int) error {
		switch i {
		case 0:
			<-panicked
			return nil
		case 2:
			defer close(panicked)
			panic("referrer 2 panicked")
		default:
			return fmt.Errorf("referrer %d failed", i)
		}
	})
	assert.Equal(t, 0, found)
	assert.Empty(t, errs)
}

func errStrings(errs []error) []string {
	s := []string{}
	for _, err := range errs {
		s = append(s, fmt.Sprint(err))
	}
	return s
}
