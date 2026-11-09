package verifiers

import (
	"errors"
	"math"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayloadLimit_DefaultAndOverride(t *testing.T) {
	restore := PayloadLimit()
	t.Cleanup(func() { SetPayloadLimit(restore) })

	assert.Equal(t, int64(10*1000*1000), PayloadLimit(), "the default must stay at 10 MB")

	SetPayloadLimit(50 * 1000 * 1000)
	assert.Equal(t, int64(50*1000*1000), PayloadLimit())

	// A non-positive value is how the flag spells "leave the default alone".
	SetPayloadLimit(0)
	assert.Equal(t, int64(50*1000*1000), PayloadLimit())
	SetPayloadLimit(-1)
	assert.Equal(t, int64(50*1000*1000), PayloadLimit())
}

// A payload between the old and the raised limit must be readable once raised:
// this is the SBOM attestation case the compile-time cap denied outright.
func TestReadLayerLimited_RaisedLimitAcceptsLargerPayload(t *testing.T) {
	const payload = 12 * 1000 * 1000

	blob := gzipBytes(t, payload)

	_, err := ReadLayerLimited(static.NewLayer(blob, types.MediaType("test")), 10*1000*1000)
	require.Error(t, err, "the default limit must still refuse it")
	assert.True(t, errors.Is(err, ErrLayerTooLarge), "got %v", err)

	b, err := ReadLayerLimited(static.NewLayer(blob, types.MediaType("test")), 20*1000*1000)
	require.NoError(t, err, "a raised limit must accept it")
	assert.Len(t, b, payload)
}

// Size refusals must be distinguishable from malformed content, so callers that
// skip unusable referrers can still tell the operator a real payload was refused.
func TestReadLayerLimited_SizeRefusalsAreTagged(t *testing.T) {
	t.Run("declared size", func(t *testing.T) {
		_, err := ReadLayerLimited(static.NewLayer(make([]byte, 2048), types.MediaType("test")), 1024)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrLayerTooLarge), "got %v", err)
	})

	t.Run("uncompressed size", func(t *testing.T) {
		_, err := ReadLayerLimited(static.NewLayer(gzipBytes(t, 4096), types.MediaType("test")), 1024)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrLayerTooLarge), "got %v", err)
	})

	t.Run("zstd window", func(t *testing.T) {
		frame := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x98, 0x01, 0x00, 0x00}
		_, err := ReadLayerLimited(static.NewLayer(frame, types.MediaType("test")), 1024)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrLayerTooLarge), "got %v", err)
		assert.True(t, errors.Is(err, zstd.ErrWindowSizeExceeded), "the cause must survive wrapping: %v", err)
	})

	t.Run("zstd declared content size", func(t *testing.T) {
		frame := []byte{
			0x28, 0xB5, 0x2F, 0xFD, // magic
			0b1110_0000,                  // FHD: FCS size flag = 3 (8 bytes), single segment
			0, 0, 0, 0, 0, 0, 0x01, 0x00, // frame content size = 1<<40
		}
		_, err := ReadLayerLimited(static.NewLayer(frame, types.MediaType("test")), 1024)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrLayerTooLarge), "got %v", err)
		assert.True(t, errors.Is(err, zstd.ErrDecoderSizeExceeded), "the cause must survive wrapping: %v", err)
	})

	t.Run("a limit at MaxInt64 still reads the payload", func(t *testing.T) {
		b, err := ReadLayerLimited(static.NewLayer(gzipBytes(t, 4096), types.MediaType("test")), math.MaxInt64)
		require.NoError(t, err)
		assert.Len(t, b, 4096)
	})

	t.Run("malformed content is not a size refusal", func(t *testing.T) {
		bad := append([]byte{0x1f, 0x8b}, make([]byte, 32)...)
		_, err := ReadLayerLimited(static.NewLayer(bad, types.MediaType("test")), 1024)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrLayerTooLarge), "got %v", err)
	})
}
