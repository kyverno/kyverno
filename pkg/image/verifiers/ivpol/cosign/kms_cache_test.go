package cosign

import (
	"context"
	"crypto"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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
	started := make(chan struct{})
	proceed := make(chan struct{})
	c := newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
		close(started)
		<-proceed
		assert.NoError(t, ctx.Err())
		return &fakeKMSVerifier{}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := c.get(ctx, "gcpkms://k", crypto.SHA256)
		errCh <- err
	}()
	<-started
	cancel() // the request ends while the build is still running
	require.ErrorIs(t, <-errCh, context.Canceled)
	close(proceed)
	v, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	require.NoError(t, err)
	assert.NotNil(t, v)
}

func TestKMSVerifierCache_WaiterHonorsContext(t *testing.T) {
	release := make(chan struct{})
	c := newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
		<-release
		return &fakeKMSVerifier{}, nil
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := c.get(ctx, "gcpkms://k", crypto.SHA256)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestKMSVerifierCache_StalledBuildTimesOutAndRetries(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	defer close(release)
	c := newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
		if calls.Add(1) == 1 {
			<-release
		}
		return &fakeKMSVerifier{}, nil
	})
	c.timeout = 20 * time.Millisecond
	_, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	require.ErrorContains(t, err, "timed out")
	v, err := c.get(context.Background(), "gcpkms://k", crypto.SHA256)
	require.NoError(t, err)
	assert.NotNil(t, v)
	assert.EqualValues(t, 2, calls.Load())
}

func TestCheckOptions_KMS(t *testing.T) {
	var gotHash atomic.Value
	var calls atomic.Int32
	old := kmsVerifiers
	t.Cleanup(func() { kmsVerifiers = old })
	kmsVerifiers = newKMSVerifierCache(func(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
		calls.Add(1)
		gotHash.Store(hash)
		if ref == "gcpkms://bad" {
			return nil, errors.New("boom")
		}
		return &fakeKMSVerifier{}, nil
	})
	baseROpts, baseNOpts := baseOpts()
	cfg := func(ref, h string) *v1beta1.Cosign {
		return &v1beta1.Cosign{
			Key:   &v1beta1.Key{KMS: ref, HashAlgorithm: h},
			CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
		}
	}
	o1, err := checkOptions(context.Background(), cfg("gcpkms://k", "sha512"), baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	o2, err := checkOptions(context.Background(), cfg("gcpkms://k", "sha512"), baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	assert.Same(t, o1.SigVerifier, o2.SigVerifier)
	assert.EqualValues(t, 1, calls.Load())
	assert.Equal(t, crypto.SHA512, gotHash.Load())

	_, err = checkOptions(context.Background(), cfg("gcpkms://bad", ""), baseROpts, baseNOpts, nil)
	require.ErrorContains(t, err, "failed to load public key from gcpkms://bad")
}

func TestKMSVerifierCache_ConcurrentDistinctKeys(t *testing.T) {
	var calls atomic.Int32
	c := countingCache(&calls, nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		for j := 0; j < 5; j++ {
			wg.Add(1)
			go func(ref string) {
				defer wg.Done()
				_, err := c.get(context.Background(), ref, crypto.SHA256)
				assert.NoError(t, err)
			}("gcpkms://k" + strconv.Itoa(i))
		}
	}
	wg.Wait()
	assert.EqualValues(t, 20, calls.Load())
}
