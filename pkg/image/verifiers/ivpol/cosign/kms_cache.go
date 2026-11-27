package cosign

import (
	"context"
	"crypto"
	"fmt"
	"sync"
	"time"

	sigs "github.com/sigstore/cosign/v3/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature"
)

type kmsBuildFunc func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error)

// kmsBuildTimeout bounds how long a caller waits for a client to be built.
// Client construction may block on the network and some providers ignore
// context cancellation, so the wait itself must be bounded.
const kmsBuildTimeout = 30 * time.Second

// kmsVerifierKey identifies a KMS verifier. The same reference with a
// different hash algorithm yields a different verifier.
type kmsVerifierKey struct {
	ref  string
	hash crypto.Hash
}

// kmsBuildCall is one in-flight client construction shared by all waiters.
type kmsBuildCall struct {
	done     chan struct{}
	verifier signature.Verifier
	err      error
}

type kmsVerifierEntry struct {
	verifier signature.Verifier
	inflight *kmsBuildCall
}

// kmsVerifierCache shares KMS verifiers across verifications and policies.
// KMS clients hold connections and background goroutines and cannot be closed,
// so building one per verification leaks resources.
type kmsVerifierCache struct {
	mu      sync.Mutex
	entries map[kmsVerifierKey]*kmsVerifierEntry
	build   kmsBuildFunc
	timeout time.Duration
}

// newKMSVerifierCache returns an empty cache that builds verifiers with build.
func newKMSVerifierCache(build kmsBuildFunc) *kmsVerifierCache {
	return &kmsVerifierCache{
		entries: map[kmsVerifierKey]*kmsVerifierEntry{},
		build:   build,
		timeout: kmsBuildTimeout,
	}
}

var kmsVerifiers = newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
	return sigs.PublicKeyFromKeyRefWithHashAlgo(ctx, ref, hash)
})

// get returns the cached verifier for (ref, hash), building it on first use.
// Concurrent first calls for the same key share one build. Failures are not
// cached, so a later call retries. Waiting is bounded by ctx and by the cache
// timeout; a build that stalls past the timeout is abandoned so a later call
// can start a new one.
func (c *kmsVerifierCache) get(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
	key := kmsVerifierKey{ref: ref, hash: hash}
	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		e = &kmsVerifierEntry{}
		c.entries[key] = e
	}
	if e.verifier != nil {
		v := e.verifier
		c.mu.Unlock()
		return v, nil
	}
	call := e.inflight
	if call == nil {
		call = &kmsBuildCall{done: make(chan struct{})}
		e.inflight = call
		// The client outlives this request, so the build must not capture a
		// context that is cancelled when the request ends.
		go c.run(context.WithoutCancel(ctx), e, call, ref, hash)
	}
	c.mu.Unlock()

	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	select {
	case <-call.done:
		return call.verifier, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		c.mu.Lock()
		if e.inflight == call {
			e.inflight = nil
		}
		c.mu.Unlock()
		return nil, fmt.Errorf("timed out after %s building KMS verifier", c.timeout)
	}
}

// run performs one build and publishes its result to every waiter. A result
// from an abandoned (timed-out) build is kept only if no verifier exists yet.
func (c *kmsVerifierCache) run(ctx context.Context, e *kmsVerifierEntry, call *kmsBuildCall, ref string, hash crypto.Hash) {
	v, err := c.build(ctx, ref, hash)
	c.mu.Lock()
	if err == nil {
		if e.verifier == nil {
			e.verifier = v
		}
		v = e.verifier
	}
	if e.inflight == call {
		e.inflight = nil
	}
	c.mu.Unlock()
	call.verifier, call.err = v, err
	close(call.done)
}
