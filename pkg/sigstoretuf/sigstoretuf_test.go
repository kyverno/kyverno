package sigstoretuf_test

import (
	"context"
	"sync"
	"testing"

	"github.com/kyverno/kyverno/pkg/sigstoretuf"
	"github.com/stretchr/testify/assert"
)

// Exercise concurrent compatibility calls with a canceled context, avoiding
// public network access while checking cancellation and shared-cache locking.
func TestConcurrentAccess(t *testing.T) {
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.ErrorIs(t, sigstoretuf.Initialize(ctx, "", nil), context.Canceled)
			_, err := sigstoretuf.TrustedRoot(ctx)
			assert.ErrorIs(t, err, context.Canceled)
			_, err = sigstoretuf.RekorPublicKeys(ctx)
			assert.ErrorIs(t, err, context.Canceled)
			_, err = sigstoretuf.CTLogPublicKeys(ctx)
			assert.ErrorIs(t, err, context.Canceled)
			_, _, err = sigstoretuf.FulcioRootsWithContext(ctx)
			assert.ErrorIs(t, err, context.Canceled)
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
