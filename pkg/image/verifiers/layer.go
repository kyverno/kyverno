package verifiers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"math"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/klauspost/compress/zstd"
)

var (
	gzipMagic = []byte{0x1f, 0x8b}
	zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}
)

// ErrLayerTooLarge marks a read refused by the size limit rather than by malformed
// content, so callers that skip unreadable referrers can still surface the refusal:
// a bundle or attestation rejected for size is an operator problem, not a non-match.
var ErrLayerTooLarge = errors.New("layer exceeds the configured size limit")

// ReadLayerLimited reads a layer's contents through a decompressor whose window and output are both bounded by maxSize, rejecting oversized or over-window content.
func ReadLayerLimited(layer v1.Layer, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("invalid layer size limit %d", maxSize)
	}

	size, err := layer.Size()
	if err != nil {
		return nil, err
	}
	if size > maxSize {
		return nil, fmt.Errorf("%w: layer size %d exceeds %d", ErrLayerTooLarge, size, maxSize)
	}

	rc, err := layer.Compressed()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	br := bufio.NewReaderSize(rc, 4)
	magic, _ := br.Peek(4)

	var reader io.Reader
	switch {
	case bytes.HasPrefix(magic, gzipMagic):
		gr, err := gzip.NewReader(br)
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		reader = gr
	case bytes.HasPrefix(magic, zstdMagic):
		dr, err := zstd.NewReader(br, zstd.WithDecoderMaxWindow(uint64(maxSize)), zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, err
		}
		defer dr.Close()
		reader = dr
	default:
		reader = br
	}

	// maxSize+1 is how an over-limit read is detected, so it must not overflow.
	probe := maxSize
	if probe == math.MaxInt64 {
		probe--
	}
	b, err := io.ReadAll(io.LimitReader(reader, probe+1))
	if err != nil {
		// Both are the decoder refusing a declared size, not malformed content:
		// a single-segment frame reports ErrDecoderSizeExceeded, a windowed one
		// ErrWindowSizeExceeded.
		if errors.Is(err, zstd.ErrWindowSizeExceeded) || errors.Is(err, zstd.ErrDecoderSizeExceeded) {
			return nil, fmt.Errorf("%w: %w", ErrLayerTooLarge, err)
		}
		return nil, fmt.Errorf("failed to read layer: %w", err)
	}
	if int64(len(b)) > maxSize {
		return nil, fmt.Errorf("%w: uncompressed layer size exceeds %d", ErrLayerTooLarge, maxSize)
	}
	return b, nil
}
