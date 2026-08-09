package internal

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationGroupCoalescesConcurrentCalls(t *testing.T) {
	var group verificationGroup[int]
	var calls atomic.Int64
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	const workers = 50
	results := make(chan int, workers)
	for range workers {
		go func() {
			value, err := group.Do(context.Background(), "same-image", func() int {
				calls.Add(1)
				once.Do(func() { close(started) })
				<-release
				return 42
			})
			require.NoError(t, err)
			results <- value
		}()
	}

	<-started
	time.Sleep(20 * time.Millisecond)
	close(release)
	for range workers {
		assert.Equal(t, 42, <-results)
	}
	assert.Equal(t, int64(1), calls.Load())
}

func TestVerificationGroupKeepsDifferentKeysIndependent(t *testing.T) {
	var group verificationGroup[string]
	var calls atomic.Int64

	first, err := group.Do(context.Background(), "first", func() string {
		calls.Add(1)
		return "one"
	})
	require.NoError(t, err)
	second, err := group.Do(context.Background(), "second", func() string {
		calls.Add(1)
		return "two"
	})
	require.NoError(t, err)

	assert.Equal(t, "one", first)
	assert.Equal(t, "two", second)
	assert.Equal(t, int64(2), calls.Load())
}

func TestVerificationGroupAllowsWaiterCancellation(t *testing.T) {
	var group verificationGroup[int]
	started := make(chan struct{})
	release := make(chan struct{})

	leaderDone := make(chan error, 1)
	go func() {
		_, err := group.Do(context.Background(), "image", func() int {
			close(started)
			<-release
			return 7
		})
		leaderDone <- err
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	waiterDone := make(chan error, 1)
	var waiterCalled atomic.Bool
	go func() {
		_, err := group.Do(ctx, "image", func() int {
			waiterCalled.Store(true)
			return 0
		})
		waiterDone <- err
	}()
	cancel()
	assert.ErrorIs(t, <-waiterDone, context.Canceled)

	close(release)
	require.NoError(t, <-leaderDone)
	assert.False(t, waiterCalled.Load())
}
