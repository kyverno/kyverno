package sigstoretuf_test

import (
	"context"
	"sync"
	"testing"

	"github.com/kyverno/kyverno/pkg/sigstoretuf"
)

// TestConcurrentAccess exercises all exported sigstoretuf functions from
// multiple goroutines simultaneously.  The test is intended to be run with
// the race detector (-race flag) to surface data races in the shared sigstore
// TUF singleton.  Network/TUF errors are expected in unit-test environments
// and are intentionally ignored; only data races are failures.
func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			// Each of these may fail with a TUF network error in unit-test
			// environments, which is fine. What matters is the absence of
			// data races reported by the race detector.
			_ = sigstoretuf.Initialize(ctx, "", nil)
			_, _ = sigstoretuf.TrustedRoot(ctx)
			_, _ = sigstoretuf.RekorPublicKeys(ctx)
			_, _ = sigstoretuf.CTLogPublicKeys(ctx)
			_, _, _ = sigstoretuf.FulcioRoots()
		}()
	}

	wg.Wait()
}

// TestWithLockSerializes verifies that WithLock prevents concurrent execution
// of the critical section by counting increments under the lock and checking
// for the expected total with no races.
func TestWithLockSerializes(t *testing.T) {
	const goroutines = 50
	counter := 0

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if err := sigstoretuf.WithLock(func() error {
				counter++
				return nil
			}); err != nil {
				t.Errorf("WithLock returned unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if counter != goroutines {
		t.Errorf("expected counter=%d, got %d", goroutines, counter)
	}
}
