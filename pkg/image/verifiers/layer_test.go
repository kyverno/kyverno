package verifiers

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"runtime"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMaxSize = int64(1 << 20) // 1 MiB, above the 1 KB zstd minimum window

// closeTrackingLayer wraps a static layer, recording whether Compressed was called and whether the returned reader was closed.
type closeTrackingLayer struct {
	v1.Layer
	rc               *closeTrackingReadCloser
	compressedCalled bool
}

type closeTrackingReadCloser struct {
	io.Reader
	closed bool
}

func (c *closeTrackingReadCloser) Close() error {
	c.closed = true
	return nil
}

func newCloseTrackingLayer(t *testing.T, data []byte) *closeTrackingLayer {
	t.Helper()
	base := static.NewLayer(data, types.MediaType("test"))
	blob, err := base.Compressed()
	require.NoError(t, err)
	return &closeTrackingLayer{Layer: base, rc: &closeTrackingReadCloser{Reader: blob}}
}

func (l *closeTrackingLayer) Compressed() (io.ReadCloser, error) {
	l.compressedCalled = true
	return l.rc, nil
}

// sizeOverrideLayer reports a declared size independent of the wrapped layer's actual bytes.
type sizeOverrideLayer struct {
	v1.Layer
	size int64
}

func (l *sizeOverrideLayer) Size() (int64, error) {
	return l.size, nil
}

// A declared size of -1 switches off go-containerregistry's bound on the body, so
// the decoder can be fed arbitrarily many compressed bytes for almost no output.
func TestReadLayerLimited_RejectsUnknownDeclaredSize(t *testing.T) {
	base := static.NewLayer(gzipBytes(t, 16), types.MediaType("test"))
	l := &sizeOverrideLayer{Layer: base, size: -1}
	b, err := ReadLayerLimited(l, testMaxSize)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown size")
	assert.False(t, errors.Is(err, ErrLayerTooLarge), "raising the limit would not help: %v", err)
	assert.Nil(t, b)
}

// A zero-length layer stays readable: unlike an image config, an empty blob is valid.
func TestReadLayerLimited_AllowsZeroLengthLayer(t *testing.T) {
	b, err := ReadLayerLimited(static.NewLayer(nil, types.MediaType("test")), testMaxSize)
	require.NoError(t, err)
	assert.Empty(t, b)
}

func gzipBytes(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write(make([]byte, n))
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func zstdBytes(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	// small window so a fixture with output > testMaxSize trips the output cap, not the window cap.
	zw, err := zstd.NewWriter(&buf, zstd.WithWindowSize(1<<16))
	require.NoError(t, err)
	_, err = zw.Write(make([]byte, n))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestReadLayerLimited_CompressedTooLarge(t *testing.T) {
	inner := newCloseTrackingLayer(t, []byte("irrelevant"))
	layer := &sizeOverrideLayer{Layer: inner, size: testMaxSize + 1}
	b, err := ReadLayerLimited(layer, testMaxSize)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	assert.Nil(t, b)
	assert.False(t, inner.compressedCalled)
}

func TestReadLayerLimited_GzipOutputTooLarge(t *testing.T) {
	data := gzipBytes(t, int(testMaxSize)+1)
	layer := static.NewLayer(data, types.MediaType("test"))
	b, err := ReadLayerLimited(layer, testMaxSize)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	assert.Nil(t, b)
}

func TestReadLayerLimited_GzipAtLimit(t *testing.T) {
	data := gzipBytes(t, int(testMaxSize))
	layer := static.NewLayer(data, types.MediaType("test"))
	b, err := ReadLayerLimited(layer, testMaxSize)
	require.NoError(t, err)
	assert.Len(t, b, int(testMaxSize))
}

// non-vacuous: false against the unfixed layer.Uncompressed() path.
func TestReadLayerLimited_ZstdWindowInflation(t *testing.T) {
	frame := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x98, 0x01, 0x00, 0x00}
	layer := static.NewLayer(frame, types.MediaType("test"))
	b, err := ReadLayerLimited(layer, testMaxSize)
	require.Error(t, err)
	assert.True(t, errors.Is(err, zstd.ErrWindowSizeExceeded), "expected ErrWindowSizeExceeded, got %v", err)
	assert.Nil(t, b)
}

// single-segment frames reject with ErrDecoderSizeExceeded, not ErrWindowSizeExceeded.
func TestReadLayerLimited_ZstdSingleSegmentInflation(t *testing.T) {
	frame := []byte{
		0x28, 0xB5, 0x2F, 0xFD, // magic
		0b1110_0000,                  // FHD: FCS size flag = 3 (8 bytes), single segment
		0, 0, 0, 0, 0, 0, 0x01, 0x00, // frame content size = 1<<40
	}
	layer := static.NewLayer(frame, types.MediaType("test"))
	b, err := ReadLayerLimited(layer, testMaxSize)
	require.Error(t, err)
	assert.True(t, errors.Is(err, zstd.ErrDecoderSizeExceeded), "expected ErrDecoderSizeExceeded, got %v", err)
	assert.Nil(t, b)
}

func TestReadLayerLimited_ZstdOutputTooLarge(t *testing.T) {
	data := zstdBytes(t, int(testMaxSize)+1)
	layer := static.NewLayer(data, types.MediaType("test"))
	b, err := ReadLayerLimited(layer, testMaxSize)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	assert.Nil(t, b)
}

// smoke check, not a hard gate; uses the rejection fixture (~0 alloc) to avoid flaking on streaming allocation churn.
func TestReadLayerLimited_ZstdBoundedAllocation(t *testing.T) {
	frame := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x98, 0x01, 0x00, 0x00}
	layer := static.NewLayer(frame, types.MediaType("test"))

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := ReadLayerLimited(layer, testMaxSize)
	runtime.ReadMemStats(&after)

	require.Error(t, err)
	deltaMiB := float64(after.TotalAlloc-before.TotalAlloc) / 1024 / 1024
	assert.Less(t, deltaMiB, float64(1), "unexpected allocation delta: %.2f MiB", deltaMiB)
}

func TestReadLayerLimited_PlainPayload(t *testing.T) {
	data := []byte(`{"hello":"world"}`)
	layer := static.NewLayer(data, types.MediaType("test"))
	b, err := ReadLayerLimited(layer, testMaxSize)
	require.NoError(t, err)
	assert.Equal(t, data, b)
}

func TestReadLayerLimited_ClosesReader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		l := newCloseTrackingLayer(t, []byte("plain"))
		_, err := ReadLayerLimited(l, testMaxSize)
		require.NoError(t, err)
		assert.True(t, l.rc.closed)
	})
	t.Run("oversized", func(t *testing.T) {
		data := gzipBytes(t, int(testMaxSize)+1)
		base := static.NewLayer(data, types.MediaType("test"))
		blob, err := base.Compressed()
		require.NoError(t, err)
		l := &closeTrackingLayer{Layer: base, rc: &closeTrackingReadCloser{Reader: blob}}
		_, err = ReadLayerLimited(l, testMaxSize)
		require.Error(t, err)
		assert.True(t, l.rc.closed)
	})
	t.Run("window-rejected", func(t *testing.T) {
		frame := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x98, 0x01, 0x00, 0x00}
		base := static.NewLayer(frame, types.MediaType("test"))
		blob, err := base.Compressed()
		require.NoError(t, err)
		l := &closeTrackingLayer{Layer: base, rc: &closeTrackingReadCloser{Reader: blob}}
		_, err = ReadLayerLimited(l, testMaxSize)
		require.Error(t, err)
		assert.True(t, l.rc.closed)
	})
}
