package libs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/image/datacache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// fakeImageFetcher returns digest (digestA when empty), or err when set.
type fakeImageFetcher struct {
	calls  atomic.Int32
	digest string
	err    error
}

func (f *fakeImageFetcher) FetchImageData(_ context.Context, image string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	digest := f.digest
	if digest == "" {
		digest = digestA
	}
	return &imagedataloader.ImageData{
		ImageDescriptor: imagedataloader.ImageDescriptor{
			ImageReference: imagedataloader.ImageReference{
				Image:  image,
				Digest: digest,
			},
		},
	}, nil
}

// these tests set the package level cache, so they must not run in parallel.
func useImageDataCache(t *testing.T) {
	t.Helper()
	c, err := datacache.New(datacache.WithTagTTLDuration(time.Minute))
	require.NoError(t, err)
	SetImageDataCache(c)
	t.Cleanup(func() { SetImageDataCache(nil) })
}

func TestGetImageDataContext_UsesTheCache(t *testing.T) {
	useImageDataCache(t)
	f := &fakeImageFetcher{}
	cp := &contextProvider{imagedata: f}
	for range 3 {
		data, err := cp.GetImageDataContext(t.Context(), "nginx:1.25", nil)
		require.NoError(t, err)
		assert.Equal(t, "nginx:1.25", data["image"])
	}
	assert.EqualValues(t, 1, f.calls.Load())
}

func TestGetImageDataContext_WithRegistryOptionsBypassesTheCache(t *testing.T) {
	useImageDataCache(t)
	f := &fakeImageFetcher{}
	cp := &contextProvider{imagedata: f}
	opts := []remote.Option{remote.WithUserAgent("credential-a")}
	// custom options cannot be told apart, so they are neither served from nor stored in the cache.
	for range 3 {
		_, err := cp.GetImageDataContext(t.Context(), "nginx:1.25", opts)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 3, f.calls.Load())
	// ... and they leave nothing behind for a lookup without options.
	_, err := cp.GetImageDataContext(t.Context(), "nginx:1.25", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 4, f.calls.Load())
	// with the cache now warm, a lookup with options still fetches its own data.
	f.digest = digestB
	data, err := cp.GetImageDataContext(t.Context(), "nginx:1.25", opts)
	require.NoError(t, err)
	assert.EqualValues(t, 5, f.calls.Load())
	assert.Equal(t, digestB, data["digest"])
	data, err = cp.GetImageDataContext(t.Context(), "nginx:1.25", nil)
	require.NoError(t, err)
	assert.Equal(t, digestA, data["digest"], "the options lookup did not overwrite the cached entry")
}

func TestGetImageDataContext_FetchErrorsAreReturnedAndNotCached(t *testing.T) {
	useImageDataCache(t)
	f := &fakeImageFetcher{err: errors.New("registry down")}
	cp := &contextProvider{imagedata: f}
	_, err := cp.GetImageDataContext(t.Context(), "nginx:1.25", nil)
	require.ErrorIs(t, err, f.err)
	f.err = nil
	for range 2 {
		data, err := cp.GetImageDataContext(t.Context(), "nginx:1.25", nil)
		require.NoError(t, err)
		assert.Equal(t, digestA, data["digest"])
	}
	assert.EqualValues(t, 2, f.calls.Load(), "the retry fetched once and was cached")
}
