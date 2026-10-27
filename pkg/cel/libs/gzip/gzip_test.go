package gzip

import (
	"bytes"
	stdgzip "compress/gzip"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultMaxOutputBytes mirrors the fallback hard-coded in Lib; the test below pins it so
// a silent change of the default is caught.
const defaultMaxOutputBytes = 10_000_000

// kyvernoGzipB64 is base64(gzip("kyverno")) produced by a standard gzip encoder (non-zero
// mtime), i.e. not by this library's own compress(). It proves decompress() reads
// third-party gzip streams, not only its own output.
const kyvernoGzipB64 = "H4sIADP4umkAA8uuLEstyssHAOgPnDMHAAAA"

// buildEnv builds a CEL env with the bounded gzip lib plus the encoders extension (for
// base64) and the `data` / `text` variables. It returns the error instead of failing the
// test so it is safe to call from goroutines.
func buildEnv(maxOutputBytes uint64) (*cel.Env, error) {
	return cel.NewEnv(
		ext.Encoders(),
		cel.Variable("data", cel.BytesType),
		cel.Variable("text", cel.StringType),
		Lib(Latest(), maxOutputBytes),
	)
}

func newEnv(t *testing.T, maxOutputBytes uint64) *cel.Env {
	t.Helper()
	env, err := buildEnv(maxOutputBytes)
	require.NoError(t, err)
	return env
}

// evalExpr compiles and evaluates expr and returns the native result. Compile and
// evaluation errors are both returned; it never touches *testing.T.
func evalExpr(env *cel.Env, expr string, vars map[string]any) (any, error) {
	ast, issues := env.Compile(expr)
	if err := issues.Err(); err != nil {
		return nil, err
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, err
	}
	out, _, err := prg.Eval(vars)
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

// eval is evalExpr for the main test goroutine: a compile error is a test bug and fails
// the test, an evaluation error is returned so error paths can be asserted.
func eval(t *testing.T, env *cel.Env, expr string, vars map[string]any) (any, error) {
	t.Helper()
	ast, issues := env.Compile(expr)
	require.NoError(t, issues.Err(), "compile %q", expr)
	prg, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := prg.Eval(vars)
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

// gzipBytes compresses s with the standard library, independent of the code under test.
func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := stdgzip.NewWriter(&buf)
	_, err := w.Write([]byte(s))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestLibraryMetadata(t *testing.T) {
	assert.NotNil(t, Latest())
	assert.Equal(t, "kyverno.gzip", (&lib{}).LibraryName())
	assert.Empty(t, (&lib{}).ProgramOptions())
}

func TestLibNilVersionPanics(t *testing.T) {
	assert.PanicsWithValue(t, "kyverno.gzip: library version must not be nil", func() {
		Lib(nil, 1)
	})
}

func TestDecompress(t *testing.T) {
	env := newEnv(t, 0)
	tests := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "ascii", in: "kyverno"},
		{name: "multi-byte utf-8", in: "xin chào Kyverno ✓"},
		{name: "newlines and tabs", in: "line1\nline2\r\n\tindented"},
		{name: "compressible 1 MiB", in: strings.Repeat("a", 1<<20)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eval(t, env, `gzip.decompress(data)`, map[string]any{"data": gzipBytes(t, tt.in)})
			require.NoError(t, err)
			assert.Equal(t, tt.in, got)
		})
	}
}

func TestDecompressStandardGzipFixture(t *testing.T) {
	env := newEnv(t, 0)
	got, err := eval(t, env, `gzip.decompress(base64.decode("`+kyvernoGzipB64+`"))`, nil)
	require.NoError(t, err)
	assert.Equal(t, "kyverno", got)
}

func TestDecompressRejectsMalformedInput(t *testing.T) {
	env := newEnv(t, 0)
	valid := gzipBytes(t, strings.Repeat("kyverno", 100))
	corrupted := bytes.Clone(valid)
	// Flip bytes in the CRC32/ISIZE trailer so the deflate stream is intact but the
	// checksum no longer matches.
	corrupted[len(corrupted)-5] ^= 0xff
	tests := []struct {
		name    string
		in      []byte
		wantErr string
	}{
		{name: "empty input", in: []byte{}, wantErr: "EOF"},
		{name: "not gzip", in: []byte("definitely not gzip"), wantErr: "invalid header"},
		{name: "header only truncated", in: valid[:4], wantErr: "EOF"},
		{name: "body truncated", in: valid[:len(valid)/2], wantErr: "unexpected EOF"},
		{name: "checksum mismatch", in: corrupted, wantErr: "invalid checksum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := eval(t, env, `gzip.decompress(data)`, map[string]any{"data": tt.in})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			// A malformed stream is an ordinary decompression error, not a cap refusal.
			assert.False(t, errors.Is(err, ErrOutputTooLarge))
		})
	}
}

func TestDecompressOutputCap(t *testing.T) {
	const limit = 1024
	env := newEnv(t, limit)
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "well below cap", size: 1},
		{name: "one byte below cap", size: limit - 1},
		{name: "exactly at cap", size: limit},
		{name: "one byte above cap", size: limit + 1, wantErr: true},
		{name: "far above cap", size: 100 * limit, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := strings.Repeat("k", tt.size)
			got, err := eval(t, env, `gzip.decompress(data)`, map[string]any{"data": gzipBytes(t, payload)})
			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrOutputTooLarge), "got: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, payload, got)
		})
	}
}

func TestDefaultOutputCap(t *testing.T) {
	// maxOutputBytes == 0 must fall back to the 10 MB default rather than "no output allowed"
	// or "unlimited".
	env := newEnv(t, 0)

	atCap := strings.Repeat("k", defaultMaxOutputBytes)
	got, err := eval(t, env, `gzip.decompress(data).size()`, map[string]any{"data": gzipBytes(t, atCap)})
	require.NoError(t, err)
	assert.EqualValues(t, defaultMaxOutputBytes, got)

	overCap := strings.Repeat("k", defaultMaxOutputBytes+1)
	_, err = eval(t, env, `gzip.decompress(data).size()`, map[string]any{"data": gzipBytes(t, overCap)})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOutputTooLarge), "got: %v", err)
}

// TestOutputCapIsPerEnv guards the design note on impl: the cap lives on the lib instance,
// not in a package-level variable, so environments built with different caps (possibly
// concurrently) must not influence each other. Run with -race to also catch a shared write.
func TestOutputCapIsPerEnv(t *testing.T) {
	const payloadSize = 50
	payload := gzipBytes(t, strings.Repeat("k", payloadSize))

	vars := map[string]any{"data": payload}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tight, err := buildEnv(payloadSize - 1)
			if !assert.NoError(t, err) {
				return
			}
			loose, err := buildEnv(payloadSize)
			if !assert.NoError(t, err) {
				return
			}

			_, err = evalExpr(tight, `gzip.decompress(data)`, vars)
			assert.True(t, errors.Is(err, ErrOutputTooLarge), "tight env, got: %v", err)

			got, err := evalExpr(loose, `gzip.decompress(data).size()`, vars)
			assert.NoError(t, err)
			assert.EqualValues(t, payloadSize, got)
		}()
	}
	wg.Wait()
}

func TestDecompressRequiresBytes(t *testing.T) {
	env := newEnv(t, 0)
	// The overload is bytes -> string; a string argument must be rejected at compile time.
	_, issues := env.Compile(`gzip.decompress(text)`)
	require.Error(t, issues.Err())
}

func TestCompress(t *testing.T) {
	env := newEnv(t, 0)

	t.Run("output is a valid gzip stream that round-trips", func(t *testing.T) {
		for _, in := range []string{"", "kyverno", "xin chào ✓", strings.Repeat("ab", 10_000)} {
			got, err := eval(t, env, `gzip.compress(text)`, map[string]any{"text": in})
			require.NoError(t, err)
			compressed, ok := got.([]byte)
			require.True(t, ok, "want []byte, got %T", got)

			r, err := stdgzip.NewReader(bytes.NewReader(compressed))
			require.NoError(t, err)
			var out bytes.Buffer
			_, err = out.ReadFrom(r)
			require.NoError(t, err)
			assert.Equal(t, in, out.String())
		}
	})

	t.Run("is deterministic", func(t *testing.T) {
		// Admission decisions and reports must not change between evaluations of the same
		// expression, so the header must not carry a wall-clock mtime.
		first, err := eval(t, env, `gzip.compress(text)`, map[string]any{"text": "kyverno"})
		require.NoError(t, err)
		second, err := eval(t, env, `gzip.compress(text)`, map[string]any{"text": "kyverno"})
		require.NoError(t, err)
		assert.Equal(t, first, second)

		header := first.([]byte)
		require.GreaterOrEqual(t, len(header), 10)
		assert.Equal(t, []byte{0x1f, 0x8b}, header[:2], "gzip magic")
		assert.Equal(t, []byte{0, 0, 0, 0}, header[4:8], "MTIME must be zero")
	})

	t.Run("rejects non-string argument", func(t *testing.T) {
		// compress is declared over dyn, so this compiles and must fail at evaluation.
		_, err := eval(t, env, `gzip.compress(1)`, nil)
		require.Error(t, err)
	})
}

func TestCompressDecompressRoundTrip(t *testing.T) {
	env := newEnv(t, 0)
	for _, in := range []string{"", "kyverno", "xin chào ✓", strings.Repeat("round-trip ", 1000)} {
		got, err := eval(t, env, `gzip.decompress(gzip.compress(text))`, map[string]any{"text": in})
		require.NoError(t, err)
		assert.Equal(t, in, got)
	}
}
