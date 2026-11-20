package compiler

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kyverno/sdk/extensions/cel/libs/http"
)

// httpCallTimeout bounds each CEL http.Get/http.Post call, in nanoseconds; zero disables the bound.
var httpCallTimeout atomic.Int64

// maxAbandonedHTTPCalls caps the calls that timed out but are still waiting on their endpoint.
// The SDK's Get and Post take no context, so a timed-out call cannot be cancelled: its goroutine
// and connection stay open until the endpoint answers or closes the connection. Past the cap, new
// calls fail at once instead of piling up more open connections. A variable so tests can lower it.
var maxAbandonedHTTPCalls int64 = 32

// abandonedHTTPCalls counts the timed-out calls that are still running.
var abandonedHTTPCalls atomic.Int64

// SetHTTPCallTimeout bounds every CEL http.Get and http.Post call made by policies compiled in
// this process, including calls through http.Client(caBundle). Zero, the default, leaves calls
// unbounded.
//
// The SDK's HTTP context sends requests without a context deadline and its guarded clients have
// no overall timeout, so an endpoint that sends headers and then stalls its body would block the
// evaluation for as long as the connection stays open. When the bound is reached, the call returns
// an error and the evaluation carries on; the abandoned request finishes in the background when
// the remote end closes the connection.
func SetHTTPCallTimeout(d time.Duration) {
	httpCallTimeout.Store(int64(d))
}

// boundedHTTPContext applies httpCallTimeout to the calls of inner.
type boundedHTTPContext struct {
	inner http.ContextInterface
}

func (b boundedHTTPContext) Get(url string, headers map[string]string) (any, error) {
	return withHTTPCallTimeout(func() (any, error) { return b.inner.Get(url, headers) })
}

func (b boundedHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	return withHTTPCallTimeout(func() (any, error) { return b.inner.Post(url, data, headers) })
}

func (b boundedHTTPContext) Client(caBundle string) (http.ContextInterface, error) {
	inner, err := b.inner.Client(caBundle)
	if err != nil {
		return nil, err
	}
	return boundedHTTPContext{inner: inner}, nil
}

func withHTTPCallTimeout(call func() (any, error)) (any, error) {
	timeout := time.Duration(httpCallTimeout.Load())
	if timeout <= 0 {
		return call()
	}
	if n := abandonedHTTPCalls.Load(); n >= maxAbandonedHTTPCalls {
		return nil, fmt.Errorf("http call refused: %d earlier calls timed out and are still waiting on their endpoints", n)
	}
	type result struct {
		value any
		err   error
	}
	var (
		mu        sync.Mutex
		finished  bool
		abandoned bool
	)
	done := make(chan result, 1)
	go func() {
		value, err := call()
		done <- result{value, err}
		mu.Lock()
		defer mu.Unlock()
		finished = true
		if abandoned {
			abandonedHTTPCalls.Add(-1)
		}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.value, r.err
	case <-timer.C:
		mu.Lock()
		defer mu.Unlock()
		if !finished {
			abandoned = true
			abandonedHTTPCalls.Add(1)
		}
		return nil, fmt.Errorf("http call did not complete within %s", timeout)
	}
}
