package gzip

import (
	"bytes"
	stdgzip "compress/gzip"
	"errors"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := stdgzip.NewWriter(&buf)
	_, err := w.Write([]byte(s))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// eval compiles expr in an env carrying only this library (with the given cap) and a `blob`
// bytes variable, then runs it.
func eval(t *testing.T, maxOutputBytes uint64, expr string, blob []byte) (ref.Val, error) {
	t.Helper()
	env, err := cel.NewEnv(Lib(Latest(), maxOutputBytes), cel.Variable("blob", cel.BytesType))
	require.NoError(t, err)
	ast, iss := env.Compile(expr)
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := prg.Eval(map[string]any{"blob": blob})
	return out, err
}

func refusedForSize(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOutputTooLarge) || strings.Contains(err.Error(), ErrOutputTooLarge.Error()),
		"expected the size cap to refuse, got: %v", err)
}

func TestDecompress_RoundTripsCompress(t *testing.T) {
	const payload = `{"apiVersion":"v1","kind":"ConfigMap","data":{"key":"value"}}`
	out, err := eval(t, 0, `gzip.decompress(gzip.compress("`+strings.ReplaceAll(payload, `"`, `\"`)+`"))`, nil)
	require.NoError(t, err)
	assert.Equal(t, payload, out.Value())
}

func TestDecompress_ReadsExternallyCompressedData(t *testing.T) {
	// Policies decompress data produced outside CEL (an annotation, a ConfigMap binaryData
	// field), so the standard library's gzip output must be readable, not only gzip.compress's.
	out, err := eval(t, 0, `gzip.decompress(blob)`, gzipBytes(t, "hello kyverno"))
	require.NoError(t, err)
	assert.Equal(t, "hello kyverno", out.Value())
}

func TestDecompress_OutputExactlyAtTheCapIsAllowed(t *testing.T) {
	const limit = 1024
	out, err := eval(t, limit, `gzip.decompress(blob)`, gzipBytes(t, strings.Repeat("a", limit)))
	require.NoError(t, err, "output of exactly maxOutputBytes must be allowed")
	assert.Len(t, out.Value(), limit)
}

func TestDecompress_OneByteOverTheCapIsRefused(t *testing.T) {
	const limit = 1024
	_, err := eval(t, limit, `gzip.decompress(blob)`, gzipBytes(t, strings.Repeat("a", limit+1)))
	refusedForSize(t, err)
}

func TestDecompress_ZipBombIsRefusedWithoutInflatingIt(t *testing.T) {
	// 50 MB of one byte compresses to about 50 KB. The cap must refuse it rather than allocate
	// the whole payload, which is the attack the bounded copy of this library exists to stop.
	bomb := gzipBytes(t, strings.Repeat("k", 50_000_000))
	require.Less(t, len(bomb), 100_000, "the test payload must actually be a small compressed input")

	_, err := eval(t, 1_000_000, `gzip.decompress(blob)`, bomb)
	refusedForSize(t, err)
}

func TestLib_ZeroCapFallsBackToTenMegabytes(t *testing.T) {
	// 0 means "use the default", not "no output allowed" and not "unlimited".
	const defaultCap = 10_000_000

	out, err := eval(t, 0, `gzip.decompress(blob).size()`, gzipBytes(t, strings.Repeat("a", defaultCap)))
	require.NoError(t, err, "output at the default cap must be allowed")
	assert.Equal(t, int64(defaultCap), out.Value())

	_, err = eval(t, 0, `gzip.decompress(blob)`, gzipBytes(t, strings.Repeat("a", defaultCap+1)))
	refusedForSize(t, err)
}

func TestDecompress_InvalidInputIsAnErrorNotAPanic(t *testing.T) {
	for name, blob := range map[string][]byte{
		"not gzip":       []byte("plain text, no gzip header"),
		"empty":          {},
		"truncated gzip": gzipBytes(t, strings.Repeat("payload ", 100))[:20],
	} {
		t.Run(name, func(t *testing.T) {
			_, err := eval(t, 0, `gzip.decompress(blob)`, blob)
			require.Error(t, err)
			assert.False(t, errors.Is(err, ErrOutputTooLarge), "a malformed input is not a size refusal: %v", err)
		})
	}
}

func TestCompress_IsDeterministic(t *testing.T) {
	// compress zeroes the gzip header's ModTime so the same input always gives the same bytes;
	// a policy comparing a compressed value against a stored one depends on that.
	first, err := eval(t, 0, `gzip.compress("same input")`, nil)
	require.NoError(t, err)
	second, err := eval(t, 0, `gzip.compress("same input")`, nil)
	require.NoError(t, err)
	assert.Equal(t, first.Value(), second.Value())

	// Bytes 4-7 of a gzip header are MTIME (RFC 1952); zero means "no timestamp". A wall-clock
	// ModTime would make two compressions of the same input differ.
	header := first.Value().([]byte)
	require.GreaterOrEqual(t, len(header), 10)
	assert.Equal(t, []byte{0, 0, 0, 0}, header[4:8], "the gzip header must carry no modification time")
}

func TestCompress_NonStringArgumentIsAnError(t *testing.T) {
	_, err := eval(t, 0, `gzip.compress(42)`, nil)
	assert.Error(t, err)
}

func TestImpl_DecompressReturnsTheSentinelError(t *testing.T) {
	// pkg/cel/compiler's IsResourceError matches ErrOutputTooLarge with errors.Is, so the value
	// the binding returns must wrap the sentinel itself, not just repeat its text.
	i := &impl{Adapter: types.DefaultTypeAdapter, maxOutputBytes: 8}
	out := i.decompress_string(types.Bytes(gzipBytes(t, "more than eight bytes")))

	errVal, ok := out.(*types.Err)
	require.True(t, ok, "expected a CEL error value, got %T", out)
	assert.ErrorIs(t, errVal, ErrOutputTooLarge)
}

func TestLib_NilVersionPanics(t *testing.T) {
	assert.PanicsWithValue(t, "kyverno.gzip: library version must not be nil", func() {
		Lib(nil, 0)
	})
}
