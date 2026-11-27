package cosign

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/url"
	"syscall"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
)

const (
	// fetchAttempts bounds how many times one registry read is tried.
	fetchAttempts       = 3
	fetchInitialBackoff = 100 * time.Millisecond
	fetchBackoffFactor  = 2
	// maxRetriesPerLoad caps the retries spent across all referrers of one image,
	// so several failing referrers cannot each use a full set of attempts.
	maxRetriesPerLoad = 4
	// minTimeLeftToRetry is how much of the request deadline must remain before
	// another attempt is started.
	minTimeLeftToRetry = 2 * time.Second
)

// retryBudget is the number of retries still allowed for one loadBundles call.
// It is used from a single goroutine. A nil budget allows no retries.
type retryBudget struct{ remaining int }

func newRetryBudget(n int) *retryBudget { return &retryBudget{remaining: n} }

func (b *retryBudget) take() bool {
	if b == nil || b.remaining <= 0 {
		return false
	}
	b.remaining--
	return true
}

// isTransientFetchError reports whether err is a failure while reading a registry
// response body that a second attempt can plausibly fix: a body cut short, a connection
// reset mid-download, or a read timeout. go-containerregistry already retries
// the request itself (connection errors and 408/429/5xx answers), so errors that
// carry a *url.Error or a registry *transport.Error are not retried again here.
// Size refusals, malformed content and a cancelled or expired context are
// permanent.
func isTransientFetchError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, verifiers.ErrLayerTooLarge) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var uerr *url.Error
	var terr *transport.Error
	if errors.As(err, &uerr) || errors.As(err, &terr) {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	var nerr net.Error
	return errors.As(err, &nerr) && nerr.Timeout()
}

// hasTimeToRetry is false once ctx is done or its deadline is too close for
// another attempt to be worth starting.
func hasTimeToRetry(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < minTimeLeftToRetry {
		return false
	}
	return true
}

// retryTransient runs fn up to fetchAttempts times with exponential backoff and
// jitter, retrying only transient errors, only while ctx has time left and only
// while budget allows. The wait between attempts ends as soon as ctx is done. On
// failure it returns the last value fn produced along with the last error.
func retryTransient[T any](ctx context.Context, budget *retryBudget, fn func() (T, error)) (T, error) {
	backoff := fetchInitialBackoff
	for attempt := 1; ; attempt++ {
		val, err := fn()
		if err == nil {
			return val, nil
		}
		if attempt >= fetchAttempts || !isTransientFetchError(err) || !hasTimeToRetry(ctx) || !budget.take() {
			return val, err
		}
		delay := backoff + time.Duration(rand.Int64N(int64(backoff/2)+1)) //nolint:gosec // jitter, not security
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return val, err
		case <-timer.C:
		}
		if !hasTimeToRetry(ctx) {
			return val, err
		}
		backoff *= fetchBackoffFactor
	}
}
