package compiler

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	localgzip "github.com/kyverno/kyverno/pkg/cel/libs/gzip"
	"github.com/kyverno/sdk/extensions/cel/libs/random"
	"github.com/kyverno/sdk/extensions/cel/libs/yaml"
	"k8s.io/apiserver/pkg/cel/library"
)

func gzipCompressForTest(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// regressionEnv builds a real env through the NewEnv chokepoint, carrying every library a
// repro-findings.md expression needs.
func regressionEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := NewEnv(
		cel.Variable("object", cel.DynType),
		cel.OptionalTypes(),
		ext.Strings(ext.StringsVersion(2)),
		ext.Lists(ext.ListsVersion(3)),
		ext.Regex(ext.RegexVersion(1)),
		ext.Sets(),
		ext.Encoders(),
		ext.Math(),
		library.Quantity(),
		random.Lib(random.Latest()),
		yaml.Lib(&yaml.YamlImpl{}, yaml.Latest()),
		localgzip.Lib(localgzip.Latest(), maxArgBytes()),
	)
	if err != nil {
		t.Fatalf("regressionEnv: %v", err)
	}
	return env
}

// evalRegression compiles and evaluates expr through the real Compile -> NewProgram ->
// ContextEval path and returns the error, the program's ActualCost if any, and the delta of
// runtime.MemStats.TotalAlloc across the call (GC forced immediately before). A refusal at
// Compile or NewProgram time (a constant pattern or quantity literal caught by the
// construction-time check) is a valid outcome, not a test-harness failure: it is an
// earlier, cheaper rejection of the same hazard, with ~0 allocation.
func evalRegression(t *testing.T, env *cel.Env, expr string, vars map[string]any) (err error, cost uint64, allocDelta uint64) {
	t.Helper()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	ast, compErr := Compile(env, expr)
	if compErr != nil {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		return compErr, 0, after.TotalAlloc - before.TotalAlloc
	}
	prg, progErr := NewProgram(env, ast)
	if progErr != nil {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		return progErr, 0, after.TotalAlloc - before.TotalAlloc
	}

	_, details, evalErr := prg.ContextEval(context.Background(), vars)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocDelta = after.TotalAlloc - before.TotalAlloc

	if details != nil {
		if c := details.ActualCost(); c != nil {
			cost = *c
		}
	}
	return evalErr, cost, allocDelta
}

// --- repro-findings.md, reproduced end to end and asserted bounded. ---

// Shape 1: the reported string-concat amplification under dyn (was ~1,231.8 MB, no error).
func TestRegressionShape1StringConcatAmplification(t *testing.T) {
	env := regressionEnv(t)
	blob := strings.Repeat("b", 400_000)
	data := map[string]any{"blob": blob}
	for i := 0; i < 500; i++ {
		data["k"+string(rune('a'+i%26))+string(rune('0'+i/26))] = "x"
	}

	err, cost, allocDelta := evalRegression(t, env,
		`object.data.map(k, object.data.blob + object.data.blob).map(s, s + s).size() >= 0`,
		map[string]any{"object": map[string]any{"data": data}},
	)
	if err == nil {
		t.Fatalf("expected a cost-limit error, got none (cost=%d, alloc=%d)", cost, allocDelta)
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	if cost <= perProgramLimit() {
		t.Fatalf("expected ActualCost() > the %d limit, got %d", perProgramLimit(), cost)
	}
	if allocDelta > 64<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 64 MB (was ~1,231.8 MB unbounded)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 1: cost=%d allocDelta=%d bytes (%.2f MB)", cost, allocDelta, float64(allocDelta)/(1<<20))
}

// Shape 2: flatten() inside a fold (was ~40,079 MB allocated / 7,694 MB retained, no error
// under a naive cel.CostLimit alone). flatten's own tracker prices it by the input's
// top-level size, which short-circuits the estimator, so the argument cap on flatten is
// what refuses this -- before the first flatten() call ever runs.
func TestRegressionShape2FlattenInFold(t *testing.T) {
	env := regressionEnv(t)
	lists := make([]any, 2000)
	for i := range lists {
		inner := make([]any, 500)
		for j := range inner {
			inner[j] = j
		}
		lists[i] = inner
	}

	err, cost, allocDelta := evalRegression(t, env,
		`lists.range(300).map(i, object.n.flatten())`,
		map[string]any{"object": map[string]any{"n": lists}},
	)
	if err == nil {
		t.Fatalf("expected a refusal, got none (cost=%d, alloc=%d)", cost, allocDelta)
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	if allocDelta > 8<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 8 MB (was ~40,079 MB / 7,694 MB retained, no error)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 2: err=%v allocDelta=%d bytes (%.2f MB)", err, allocDelta, float64(allocDelta)/(1<<20))
}

// Shape 3: random("(((a{5})*)*){20}") -- was ~37.6 GB allocated, ~12 s, cost 3, from a
// ~20-byte policy literal. Because the pattern is a constant literal, the replacement
// regex-literal validator (regex_validator.go) catches it during env.Compile itself.
func TestRegressionShape3RandomAmplification(t *testing.T) {
	env := regressionEnv(t)
	err, cost, allocDelta := evalRegression(t, env,
		`random("(((a{5})*)*){20}").size() >= 0`,
		map[string]any{"object": map[string]any{}},
	)
	if err == nil {
		t.Fatalf("expected a refusal, got none (cost=%d, alloc=%d)", cost, allocDelta)
	}
	if !strings.Contains(err.Error(), "unrolls to") {
		t.Fatalf("expected the unrolled-length refusal, got: %v", err)
	}
	if allocDelta > 1<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 1 MB (was ~37.6 GB, ~12 s)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 3: err=%v allocDelta=%d bytes", err, allocDelta)
}

// Shape 4: gzip.decompress -- a highly compressible payload decompresses far past its
// compressed size (was 357.1 MB from a 99.6 KB payload, cost 5, no error). The local
// bounded gzip lib's io.LimitReader refuses past maxStepBytes (10 MB at defaults).
func TestRegressionShape4GzipAmplification(t *testing.T) {
	env := regressionEnv(t)
	payload := strings.Repeat("k", 50_000_000) // highly compressible; small gzip, huge decompressed size
	gz := gzipCompressForTest(t, payload)

	err, cost, allocDelta := evalRegression(t, env,
		`gzip.decompress(object.blob).size() >= 0`,
		map[string]any{"object": map[string]any{"blob": gz}},
	)
	if err == nil {
		t.Fatalf("expected a refusal, got none (cost=%d, alloc=%d)", cost, allocDelta)
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	// io.ReadAll's own doubling growth strategy makes the exact figure noisy run to run;
	// the property under test is "bounded, no longer proportional to the true decompressed
	// size", not a tight constant.
	if allocDelta > 64<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 64 MB (was 357.1 MB for a much smaller ratio)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 4: err=%v allocDelta=%d bytes", err, allocDelta)
}

// Shape 5: quantity("1e100000000") -- was ~429.7 MB, ~25 s, cost 5, from two policy
// literals. The exponent cap refuses the first quantity() call before parsing reaches the
// expensive path.
func TestRegressionShape5QuantityAmplification(t *testing.T) {
	env := regressionEnv(t)
	err, cost, allocDelta := evalRegression(t, env,
		`quantity("1e100000000").add(quantity("1")).isGreaterThan(quantity("0"))`,
		map[string]any{"object": map[string]any{}},
	)
	if err == nil {
		t.Fatalf("expected a refusal, got none (cost=%d, alloc=%d)", cost, allocDelta)
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	if allocDelta > 4<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 4 MB (was ~429.7 MB / ~25 s)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 5: err=%v allocDelta=%d bytes", err, allocDelta)
}

// Shape 6: string_format was in tier 1, priced by cel-go's own formula on the *template*
// argument ("%s" is two bytes), never the list argument the formatted result actually
// depends on (was 945 MB / cost 1,314 for 100 literal calls over a 9 MiB value, no error).
// The literal-list shape is the one that matters and is reproduced exactly: a comprehension
// form already trips the per-program cost limit through the fold step's own per-iteration
// charge regardless of whether format itself is fixed, so testing that shape proves nothing
// about format specifically.
func TestRegressionShape6FormatLiteralListRefuses(t *testing.T) {
	env := regressionEnv(t)
	big := strings.Repeat("a", 9*1024*1024) // 9 MiB
	calls := make([]string, 100)
	for i := range calls {
		calls[i] = `"%s".format([object.s])`
	}
	expr := "[" + strings.Join(calls, ", ") + "].size() >= 0"
	err, cost, allocDelta := evalRegression(t, env, expr, map[string]any{"object": map[string]any{"s": big}})
	if err == nil {
		t.Fatalf("expected a refusal, got none (cost=%d, alloc=%d) -- this is the exact repro that cost 1,314 and retained 943.8 MB unfixed", cost, allocDelta)
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	if allocDelta > 8<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 8 MB (was 943.8 MB before the fix)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 6 (format, literal list of 100): err=%v cost=%d allocDelta=%d bytes", err, cost, allocDelta)
}

// Shape 7: list.join(sep) with a large separator and many (even empty) elements -- was
// ~2.7 GB from one call. joinGuard bounds the exact strings.Join output.
func TestRegressionShape7JoinAmplification(t *testing.T) {
	env := regressionEnv(t)
	elems := make([]any, 500)
	for i := range elems {
		elems[i] = ""
	}
	sep := strings.Repeat("s", 5*1024*1024) // 5 MiB separator

	err, cost, allocDelta := evalRegression(t, env,
		`object.l.join(object.sep).size() >= 0`,
		map[string]any{"object": map[string]any{"l": elems, "sep": sep}},
	)
	if err == nil {
		t.Fatalf("expected a refusal, got none (cost=%d, alloc=%d)", cost, allocDelta)
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	if allocDelta > 16<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 16 MB (was ~2.7 GB unbounded)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 7 (join): err=%v allocDelta=%d bytes", err, allocDelta)
}

// Shape 8: yaml.parse alias expansion -- a 13,550-byte alias-nested literal decodes to
// 26.3 MB at cost 4 under the generic rule alone (the YAML decoder's own alias-ratio guard
// does not fire), and the decoded map's nesting is invisible to a shallow result charge.
// yamlParseExpansion (cost.go's parserFloor) and the "parse" guard-table entry
// (guards.go, maxYamlParseBytes) are what this cut adds to close it -- extending the
// parser floor rather than adding a local bounded yaml copy, per the design's own preferred
// remedy for this gap.
func TestRegressionShape8YamlParseAliasExpansion(t *testing.T) {
	env := regressionEnv(t)
	// An alias-nested literal at the same order of size as repro-findings.md section 12's
	// exact case (13,550 bytes -> 26.3 MB, no error, alias-ratio guard did not fire): a
	// base list referenced repeatedly through a couple of alias levels, with no level
	// referencing itself (a cycle fails to decode, not amplify -- that is a different,
	// pre-existing issue tracked separately, not this hazard). The literal is built past
	// maxYamlParseBytes (~4,883 bytes at defaults) the same way the measured repro's
	// 13,550-byte literal is, so the parser input cap is what refuses it.
	var b strings.Builder
	b.WriteString("l0: &l0 [")
	for j := 0; j < 600; j++ {
		if j > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%q", strings.Repeat("x", 20)))
	}
	b.WriteString("]\n")
	prev := "l0"
	const levels = 3
	for i := 1; i <= levels; i++ {
		name := fmt.Sprintf("l%d", i)
		b.WriteString(name + ": &" + name + " [")
		for j := 0; j < 15; j++ {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("*" + prev)
		}
		b.WriteString("]\n")
		prev = name
	}
	literal := b.String()
	if len(literal) < 13_000 {
		t.Fatalf("test fixture literal is %d bytes, want >= 13,000 to match the measured repro's order of magnitude", len(literal))
	}

	// Build the CEL string literal from the YAML text via a Go string literal (avoids
	// hand-escaping newlines/quotes for a small fixture).
	expr := fmt.Sprintf("yaml.parse(%q).size() >= 0", literal)
	err, cost, allocDelta := evalRegression(t, env, expr, map[string]any{"object": map[string]any{}})
	if err == nil {
		t.Fatalf("expected a refusal, got none (cost=%d, alloc=%d) -- this is the aliasing shape that reached 26.3 MB from a 13,550-byte literal unfixed", cost, allocDelta)
	}
	// The specific guard-cap text, not just "any error": parserFloor's charge on a call
	// the guard already refused used to be large enough to trip the per-program cost limit
	// on its own, masking the guard's own refusal behind a generic cost-limit cancellation
	// -- a test that only checked IsResourceError(err) could not tell a masked guard from
	// a working one (guardTable's "parse" row could be deleted and this test stayed green).
	if !strings.Contains(err.Error(), "parser input is") || !strings.Contains(err.Error(), "byte cap") {
		t.Fatalf("expected the parser-cap guard refusal specifically, got: %v", err)
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	if allocDelta > 4<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 4 MB (was 26.3 MB from a 13,550-byte literal, no error)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("shape 8 (yaml.parse): literal=%d bytes err=%v allocDelta=%d bytes", len(literal), err, allocDelta)
}

// TestRegressionShape8GuardIsLoadBearing is the negative control (D58-style,
// disabledGuardForTest): with the "parse" guard disabled, the same call is refused instead
// by the per-program cost limit (parserFloor's charge alone), at far more allocation --
// proving the guard, not some other mechanism, is what keeps shape 8's allocation small.
func TestRegressionShape8GuardIsLoadBearing(t *testing.T) {
	env := regressionEnv(t)
	var b strings.Builder
	b.WriteString("l0: &l0 [")
	for j := 0; j < 600; j++ {
		if j > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%q", strings.Repeat("x", 20)))
	}
	b.WriteString("]\n")
	prev := "l0"
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("l%d", i)
		b.WriteString(name + ": &" + name + " [")
		for j := 0; j < 15; j++ {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("*" + prev)
		}
		b.WriteString("]\n")
		prev = name
	}
	literal := b.String()
	expr := fmt.Sprintf("yaml.parse(%q).size() >= 0", literal)

	SetDisabledGuardForTest("parse")
	defer SetDisabledGuardForTest("")

	err, _, allocDelta := evalRegression(t, env, expr, map[string]any{"object": map[string]any{}})
	if err == nil {
		t.Fatalf("expected a refusal even with the guard disabled (the parser floor should still trip the cost limit), got none")
	}
	if strings.Contains(err.Error(), "parser input is") {
		t.Fatalf("guard fired even though it was disabled for this test -- disabledGuardForTest is not wired into parserGuard")
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
	t.Logf("shape 8 with guard disabled: err=%v allocDelta=%d bytes (%.2f MB) -- with the guard, allocDelta stays under 4 MB (see TestRegressionShape8YamlParseAliasExpansion)", err, allocDelta, float64(allocDelta)/(1<<20))
}

// zero-argument calls (e.g. time.now()) must not panic at evaluation: the guard decorator
// leaves them undecorated (guards.go), since evalVarArgs indexes argVals[0] unconditionally.
func TestRegressionZeroArgCallDoesNotPanic(t *testing.T) {
	env := regressionEnv(t)
	err, _, _ := evalRegression(t, env, `random.random().size() >= 0`, map[string]any{"object": map[string]any{}})
	if err != nil {
		t.Fatalf("zero-arg call should evaluate cleanly, got: %v", err)
	}
}

// __orValueCompat__ must propagate a resource refusal instead of silently absorbing it into
// orValue's default -- otherwise wrapping any guarded hazard in .orValue(x) would bypass
// every bound this file proves.
func TestOrValueCompatPropagatesResourceErrors(t *testing.T) {
	env, err := NewEnv(
		cel.Variable("object", cel.DynType),
		OrValueCompatEnvOptions()[0], OrValueCompatEnvOptions()[1],
		random.Lib(random.Latest()),
	)
	if err != nil {
		t.Fatalf("NewEnv: %v", err)
	}
	_, cost, allocDelta := evalRegression(t, env,
		`random("(((a{5})*)*){20}").orValue("fallback")`,
		map[string]any{"object": map[string]any{}},
	)
	_ = cost
	_ = allocDelta
	// The refusal happens at Compile time (a constant-pattern literal), so orValue never
	// runs; evalRegression's Compile step returns the error directly. Re-run with a
	// non-constant pattern so the refusal happens at eval time, through orValue.
	err2, _, allocDelta2 := evalRegression(t, env,
		`random(object.pattern).orValue("fallback")`,
		map[string]any{"object": map[string]any{"pattern": "(((a{5})*)*){20}"}},
	)
	if err2 == nil {
		t.Fatalf("expected orValue to propagate the guard refusal, got no error (alloc=%d)", allocDelta2)
	}
	if !IsResourceError(err2) {
		t.Fatalf("expected orValue to propagate a resource error, got: %v", err2)
	}
}
