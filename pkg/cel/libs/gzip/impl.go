package gzip

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"time"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/sdk/extensions/cel/utils"
)

// ErrOutputTooLarge is a refusal, not an ordinary decompression error: the size cap fired,
// not gzip itself. Exported so pkg/cel/compiler's IsResourceError recognizes it the same
// way it recognizes a guard-table refusal.
var ErrOutputTooLarge = errors.New("gzip: decompressed output exceeds the size cap")

// impl carries the decompression cap as a field (closed over via the lib instance that
// constructs it), not a package-level variable: see Lib's doc comment for why a shared
// mutable global here would be a data race, not just an idempotent overwrite.
type impl struct {
	types.Adapter
	maxOutputBytes uint64
}

func (i *impl) decompress_string(gzBytes ref.Val) ref.Val {
	data, err := utils.ConvertToNative[[]byte](gzBytes)
	if err != nil {
		return types.WrapErr(err)
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return types.WrapErr(err)
	}
	defer r.Close()

	// Read at most maxOutputBytes+1: if the stream still has data past that, the extra
	// byte is what proves the cap was exceeded, without ever materializing more than
	// maxOutputBytes+1 bytes of decompressed output.
	limited := io.LimitReader(r, int64(i.maxOutputBytes)+1) //nolint:gosec // maxOutputBytes is an operator-configured cap (default 10_000_000), never attacker-controlled and never near 1<<63
	out, err := io.ReadAll(limited)
	if err != nil {
		return types.WrapErr(err)
	}
	if uint64(len(out)) > i.maxOutputBytes {
		return types.WrapErr(ErrOutputTooLarge)
	}
	return i.NativeToValue(string(out))
}

func (i *impl) compress(value ref.Val) ref.Val {
	native, err := utils.ConvertToNative[string](value)
	if err != nil {
		return types.WrapErr(err)
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.ModTime = time.Unix(0, 0) // to make compression deterministic

	if _, err := w.Write([]byte(native)); err != nil {
		return types.WrapErr(err)
	}
	if err := w.Close(); err != nil {
		return types.WrapErr(err)
	}
	return i.NativeToValue(buf.Bytes())
}
