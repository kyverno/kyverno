package cosign

import (
	"context"
	"crypto"
	"sync"

	sigs "github.com/sigstore/cosign/v3/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature"
)

type kmsBuildFunc func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error)

// kmsVerifierKey identifies a KMS verifier. The same reference with a
// different hash algorithm yields a different verifier.
type kmsVerifierKey struct {
	ref  string
	hash crypto.Hash
}

type kmsVerifierEntry struct {
	mu       sync.Mutex
	verifier signature.Verifier
}

// kmsVerifierCache shares KMS verifiers across verifications and policies.
// KMS clients hold connections and background goroutines and cannot be closed,
// so building one per verification leaks resources.
type kmsVerifierCache struct {
	mu      sync.Mutex
	entries map[kmsVerifierKey]*kmsVerifierEntry
	build   kmsBuildFunc
}

func newKMSVerifierCache(build kmsBuildFunc) *kmsVerifierCache {
	return &kmsVerifierCache{entries: map[kmsVerifierKey]*kmsVerifierEntry{}, build: build}
}

var kmsVerifiers = newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
	return sigs.PublicKeyFromKeyRefWithHashAlgo(ctx, ref, hash)
})

// get returns the cached verifier for (ref, hash), building it on first use.
// Concurrent first calls for the same key build one client. Failures are not
// cached, so a later call retries.
func (c *kmsVerifierCache) get(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
	key := kmsVerifierKey{ref: ref, hash: hash}
	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		e = &kmsVerifierEntry{}
		c.entries[key] = e
	}
	c.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.verifier != nil {
		return e.verifier, nil
	}
	// The client outlives this request, so it must not capture a context
	// that is cancelled when the request ends.
	v, err := c.build(context.WithoutCancel(ctx), ref, hash)
	if err != nil {
		return nil, err
	}
	e.verifier = v
	return v, nil
}
