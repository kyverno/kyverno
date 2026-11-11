package cosign

import (
	"context"
	"crypto"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeKMSVerifier struct {
	signature.Verifier
}

func countingCache(calls *atomic.Int32, fail *atomic.Bool) *kmsVerifierCache {
	return newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
		calls.Add(1)
		if fail != nil && fail.Load() {
			return nil, errors.New("transient")
		}
		return &fakeKMSVerifier{}, nil
	})
}

func TestKMSVerifierCache_ReusesSameKey(t *testing.T) {
	var calls atomic.Int32
	c := countingCache(&calls, nil)
	a, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	require.NoError(t, err)
	b, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	require.NoError(t, err)
	assert.Same(t, a, b)
	assert.EqualValues(t, 1, calls.Load())
}

func TestKMSVerifierCache_DistinctKeys(t *testing.T) {
	var calls atomic.Int32
	c := countingCache(&calls, nil)
	_, _ = c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	_, _ = c.get(context.Background(), "gcpkms://other", crypto.SHA256)
	_, _ = c.get(context.Background(), "gcpkms://k", crypto.SHA512)
	assert.EqualValues(t, 3, calls.Load())
}

func TestKMSVerifierCache_FailureNotCached(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	fail.Store(true)
	c := countingCache(&calls, &fail)
	_, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	require.Error(t, err)
	fail.Store(false)
	v, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	require.NoError(t, err)
	assert.NotNil(t, v)
	assert.EqualValues(t, 2, calls.Load())
}

func TestKMSVerifierCache_ConcurrentSingleBuild(t *testing.T) {
	var calls atomic.Int32
	c := countingCache(&calls, nil)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.EqualValues(t, 1, calls.Load())
}

func TestKMSVerifierCache_BuildContextNotCancelled(t *testing.T) {
	c := newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
		assert.NoError(t, ctx.Err())
		return &fakeKMSVerifier{}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.get(ctx, "gcpkms://k", crypto.SHA256)
	require.NoError(t, err)
}
