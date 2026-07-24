package breaker

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_concurrencyLimiter_Do(t *testing.T) {
	t.Run("zero limit returns inner unchanged", func(t *testing.T) {
		inner := NewBreaker("", nil)
		subject := WithConcurrencyLimit("", 0, inner)
		assert.Equal(t, Breaker(inner), subject)
	})
	t.Run("passes calls through under the limit", func(t *testing.T) {
		subject := WithConcurrencyLimit("", 2, NewBreaker("", nil))
		called := false
		err := subject.Do(context.TODO(), func(context.Context) error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)
	})
	t.Run("propagates inner errors", func(t *testing.T) {
		subject := WithConcurrencyLimit("", 1, NewBreaker("", nil))
		err := subject.Do(context.TODO(), func(context.Context) error {
			return errors.New("foo")
		})
		assert.Error(t, err)
	})
	t.Run("drops calls when saturated and releases slots", func(t *testing.T) {
		subject := WithConcurrencyLimit("", 1, NewBreaker("", nil))
		started := make(chan struct{})
		release := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = subject.Do(context.TODO(), func(context.Context) error {
				close(started)
				<-release
				return nil
			})
		}()
		<-started
		// saturated: the call must be dropped without invoking inner
		called := false
		err := subject.Do(context.TODO(), func(context.Context) error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.False(t, called)
		// once the slot is released, calls pass through again
		close(release)
		wg.Wait()
		err = subject.Do(context.TODO(), func(context.Context) error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)
	})
}
