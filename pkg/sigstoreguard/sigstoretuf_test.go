package sigstoreguard

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the guarded repository state without network access. Run with -race
// to check concurrent default snapshots and the shared cache lock.
func TestConcurrentAccess(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	defer SetDefaultRepository(previousMirror, previousRoot)
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	const goroutines = 10
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 10 {
				setting := fmt.Sprintf("mirror-%d-%d", i, j)
				SetDefaultRepository(setting, []byte(setting))
				mirror, rootBytes := defaultRepository()
				assert.Equal(t, mirror, string(rootBytes), "operator mirror and root must be from the same snapshot")
				assert.ErrorIs(t, Initialize(ctx, "", nil), context.Canceled)
				_, err := TrustedRoot(ctx)
				assert.ErrorIs(t, err, context.Canceled)
				_, err = TrustedRootFor(ctx, "https://policy.example", nil)
				assert.ErrorIs(t, err, context.Canceled)
				_, err = VerificationMaterialFor(ctx, "", nil)
				assert.ErrorIs(t, err, context.Canceled)
				_, err = RekorPublicKeys(ctx)
				assert.ErrorIs(t, err, context.Canceled)
				_, err = CTLogPublicKeys(ctx)
				assert.ErrorIs(t, err, context.Canceled)
				_, _, err = FulcioRootsWithContext(ctx)
				assert.ErrorIs(t, err, context.Canceled)
			}
		}()
	}
	wg.Wait()
}

func TestDefaultRepositoryOwnsRootSnapshots(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	defer SetDefaultRepository(previousMirror, previousRoot)
	input := []byte("root")
	SetDefaultRepository("operator", input)
	input[0] = 'x'
	mirror, rootBytes := defaultRepository()
	require.Equal(t, "operator", mirror)
	require.Equal(t, []byte("root"), rootBytes)
	rootBytes[0] = 'y'
	_, again := defaultRepository()
	require.Equal(t, []byte("root"), again)
}

func TestWithLockSerializes(t *testing.T) {
	const goroutines = 50
	counter := 0
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := WithLock(func() error { counter++; return nil }); err != nil {
				t.Errorf("WithLock returned unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, goroutines, counter)
}
