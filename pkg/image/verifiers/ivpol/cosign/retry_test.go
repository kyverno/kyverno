package cosign

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_isTransientFetchError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unexpected EOF", io.ErrUnexpectedEOF, true},
		{"wrapped unexpected EOF", fmt.Errorf("failed to read layer: %w", io.ErrUnexpectedEOF), true},
		{"connection reset mid-body", fmt.Errorf("read: %w", syscall.ECONNRESET), true},
		{"broken pipe", fmt.Errorf("write: %w", syscall.EPIPE), true},
		{"read timeout", fakeNetError{timeout: true}, true},
		{"wrapped read timeout", fmt.Errorf("read body: %w", fakeNetError{timeout: true}), true},
		{"net error that is not a timeout", fakeNetError{}, false},
		{"request error, already retried by the registry client", &url.Error{Op: "Get", URL: "http://registry", Err: io.ErrUnexpectedEOF}, false},
		{"http 429, already retried by the registry client", &transport.Error{StatusCode: http.StatusTooManyRequests}, false},
		{"http 503, already retried by the registry client", &transport.Error{StatusCode: http.StatusServiceUnavailable}, false},
		{"http 404", &transport.Error{StatusCode: http.StatusNotFound}, false},
		{"deadline exceeded", context.DeadlineExceeded, false},
		{"context canceled", context.Canceled, false},
		{"layer too large", fmt.Errorf("%w: layer size 10 exceeds 5", verifiers.ErrLayerTooLarge), false},
		{"plain error", errors.New("bundle version too old"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTransientFetchError(tt.err))
		})
	}
}

func Test_retryTransient_SucceedsAfterTransientFailures(t *testing.T) {
	calls := 0
	got, err := retryTransient(context.Background(), newRetryBudget(10), func() (string, error) {
		calls++
		if calls < fetchAttempts {
			return "", io.ErrUnexpectedEOF
		}
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", got)
	assert.Equal(t, fetchAttempts, calls)
}

func Test_retryTransient_StopsAtAttemptLimit(t *testing.T) {
	calls := 0
	_, err := retryTransient(context.Background(), newRetryBudget(10), func() (string, error) {
		calls++
		return "", io.ErrUnexpectedEOF
	})
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, fetchAttempts, calls)
}

func Test_retryTransient_DoesNotRetryPermanentError(t *testing.T) {
	calls := 0
	_, err := retryTransient(context.Background(), newRetryBudget(10), func() (string, error) {
		calls++
		return "", verifiers.ErrLayerTooLarge
	})
	require.ErrorIs(t, err, verifiers.ErrLayerTooLarge)
	assert.Equal(t, 1, calls)
}

func Test_retryTransient_NilBudgetAllowsNoRetries(t *testing.T) {
	calls := 0
	_, err := retryTransient(context.Background(), nil, func() (string, error) {
		calls++
		return "", io.ErrUnexpectedEOF
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

// One budget is shared by every call of a loadBundles run, so the retries a
// failing referrer uses are not available to the next one.
func Test_retryTransient_BudgetIsSharedAcrossCalls(t *testing.T) {
	budget := newRetryBudget(2)
	failing := func(calls *int) func() (string, error) {
		return func() (string, error) {
			*calls++
			return "", io.ErrUnexpectedEOF
		}
	}

	first, second := 0, 0
	_, err := retryTransient(context.Background(), budget, failing(&first))
	require.Error(t, err)
	_, err = retryTransient(context.Background(), budget, failing(&second))
	require.Error(t, err)

	assert.Equal(t, 3, first, "the first call may spend the whole budget")
	assert.Equal(t, 1, second, "the second call finds it empty and is tried once")
}

// The wait between attempts must end when the request context does. The test
// runs on a fake clock: it only advances if the wait sleeps, so the assertion
// does not depend on the scheduler.
func Test_retryTransient_StopsWaitingWhenContextCancelled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cancelAfter := fetchInitialBackoff / 10
		go func() {
			time.Sleep(cancelAfter)
			cancel()
		}()

		calls := 0
		start := time.Now()
		_, err := retryTransient(ctx, newRetryBudget(10), func() (string, error) {
			calls++
			return "", io.ErrUnexpectedEOF
		})

		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.Equal(t, 1, calls, "no further attempt once the context is done")
		assert.Equal(t, cancelAfter, time.Since(start), "the wait must end when the context is cancelled")
	})
}

// With the deadline too close, another attempt is not started at all.
func Test_retryTransient_NoRetryWhenDeadlineIsClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), minTimeLeftToRetry/4)
	defer cancel()

	calls := 0
	_, err := retryTransient(ctx, newRetryBudget(10), func() (string, error) {
		calls++
		return "", io.ErrUnexpectedEOF
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func Test_retryTransient_NoAttemptsAfterContextAlreadyDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	_, err := retryTransient(ctx, newRetryBudget(10), func() (string, error) {
		calls++
		return "", io.ErrUnexpectedEOF
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

// The deadline is checked again after the backoff wait: a wait that eats the
// margin must not be followed by another attempt.
func Test_retryTransient_NoRetryWhenBackoffUsesUpTheMargin(t *testing.T) {
	// On a fake clock the first check always passes with 50ms to spare, and the
	// wait (at least fetchInitialBackoff) then uses the margin up.
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), minTimeLeftToRetry+50*time.Millisecond)
		defer cancel()

		calls := 0
		start := time.Now()
		_, err := retryTransient(ctx, newRetryBudget(10), func() (string, error) {
			calls++
			return "", io.ErrUnexpectedEOF
		})

		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.GreaterOrEqual(t, time.Since(start), fetchInitialBackoff, "the wait must have happened")
		assert.Equal(t, 1, calls, "no attempt once the wait has used up the margin")
	})
}

// fakeNetError is a net.Error whose Timeout result is chosen by the test.
type fakeNetError struct{ timeout bool }

func (e fakeNetError) Error() string   { return "fake net error" }
func (e fakeNetError) Timeout() bool   { return e.timeout }
func (e fakeNetError) Temporary() bool { return false }

func Test_retryTransient_SucceedsAfterReadTimeout(t *testing.T) {
	calls := 0
	got, err := retryTransient(context.Background(), newRetryBudget(10), func() (string, error) {
		calls++
		if calls == 1 {
			return "", fmt.Errorf("read body: %w", fakeNetError{timeout: true})
		}
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", got)
	assert.Equal(t, 2, calls)
}
