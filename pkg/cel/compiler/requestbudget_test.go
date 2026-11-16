package compiler

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
)

// nearLimitVariableExpr concatenates a dyn (unresolved-overload) string with itself: a real
// allocation, not a reference. object.blob is dyn, so add_string's overloadID is empty and
// this falls to the generic rule, charged on both sides -- ceil(f*argBytes) (two 2.2 MB
// arguments) plus ceil(f*resultBytes) (the 4.4 MB result) -- about 880,000 at
// blobSize = 2,200,000, just under the 1,000,000 per-program limit, while still retaining a
// multi-megabyte value per evaluation: the same shape a policy with many "variables" each
// computing one derived, sizable value produces, and exactly what the per-program limit
// alone cannot bound across more than one such variable.
const (
	nearLimitVariableExpr = `object.blob + object.blob`
	nearLimitBlobSize     = 2_200_000
	nearLimitRetainedMB   = 2 * nearLimitBlobSize / (1 << 20) // one evaluation's real result size
)

func evalNearLimitVariables(t *testing.T, ctx context.Context, n int) (allocDelta uint64, refusedAt int, lastErr error) {
	t.Helper()
	return evalNearLimitVariablesSized(t, ctx, n, nearLimitBlobSize)
}

func evalNearLimitVariablesSized(t *testing.T, ctx context.Context, n, blobSize int) (allocDelta uint64, refusedAt int, lastErr error) {
	t.Helper()
	env := regressionEnv(t)
	blob := strings.Repeat("b", blobSize)
	vars := map[string]any{"object": map[string]any{"blob": blob}}

	ast, err := Compile(env, nearLimitVariableExpr)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	prg, err := NewProgram(env, ast)
	if err != nil {
		t.Fatalf("program: %v", err)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < n; i++ {
		if _, _, err := prg.ContextEval(ctx, vars); err != nil {
			refusedAt = i + 1
			lastErr = err
			break
		}
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc, refusedAt, lastErr
}

// TestRequestBudgetBoundsLinearGrowthAcrossVariables reproduces the reviewer's finding: a
// policy with about 50 "variables" (here, 50 evaluations sharing one request), each
// retaining a multi-megabyte value and individually under the per-program limit, retained
// that amount times 1 / 10 / 50 for 1 / 10 / 50 variables with no error -- linear and
// unbounded per request, since the per-program limit only ever sees one variable at a time
// (TestRequestBudgetDegradesGracefullyWithoutBudget below pins that this is still exactly
// what happens with no budget in ctx). With requestbudget.Ensure on ctx, the shared counter
// (celconfig.RuntimeCELCostBudget = 10,000,000 default) is spent after about eleven
// evaluations at ~880,000 each, so the 50th is refused well before the unbounded point.
func TestRequestBudgetBoundsLinearGrowthAcrossVariables(t *testing.T) {
	for _, n := range []int{1, 10, 50} {
		t.Run("", func(t *testing.T) {
			budgeted, _ := requestbudget.Ensure(context.Background())
			allocDelta, refusedAt, err := evalNearLimitVariables(t, budgeted, n)
			t.Logf("n=%d allocDelta=%d bytes (%.1f MB) refusedAt=%d err=%v", n, allocDelta, float64(allocDelta)/(1<<20), refusedAt, err)
			if n >= 50 {
				if refusedAt == 0 {
					t.Fatalf("n=%d: expected the shared request budget to refuse before all %d completed, all succeeded (alloc=%.1f MB) -- was ~%d MB, no error, before the request budget", n, n, float64(allocDelta)/(1<<20), n*nearLimitRetainedMB)
				}
				if !IsResourceError(err) {
					t.Fatalf("expected a resource error, got: %v", err)
				}
				if refusedAt >= n {
					t.Fatalf("expected refusal strictly before the %dth evaluation, refused at %d", n, refusedAt)
				}
			}
			// No longer linear: n variables must not retain anywhere near n times a single
			// variable's own retained size (the unfixed figure for n=50).
			if allocDelta > 128<<20 {
				t.Fatalf("n=%d: allocDelta = %d bytes (%.1f MB), want well under the unfixed linear figure (%d MB x %d = %d MB)", n, allocDelta, float64(allocDelta)/(1<<20), nearLimitRetainedMB, n, n*nearLimitRetainedMB)
			}
		})
	}
}

// TestRequestBudgetDegradesGracefullyWithoutBudget pins the deliberate trade in
// requestbudget.Charge's doc comment: a ctx with no budget (a missed Ensure call) charges
// nothing and never refuses -- today's per-program-limit-only behavior, unchanged -- rather
// than failing every evaluation the way the dropped budget package's ErrNoBudget did.
func TestRequestBudgetDegradesGracefullyWithoutBudget(t *testing.T) {
	allocDelta, refusedAt, err := evalNearLimitVariables(t, context.Background(), 50)
	if refusedAt != 0 {
		t.Fatalf("expected all 50 to succeed with no budget in ctx (graceful degradation), refused at %d: %v", refusedAt, err)
	}
	t.Logf("no budget in ctx, n=50: allocDelta=%d bytes (%.1f MB) -- unchanged, linear, today's behavior", allocDelta, float64(allocDelta)/(1<<20))
}

// eightExpressionBlobSize costs about 400,000 per evaluation (blobSize=1,000,000; the same
// dual-charge arithmetic as nearLimitVariableExpr's own doc comment: 0.4 x blobSize), so 8
// of them sum to 3.2M -- over EnsureBackground's 3,000,000 ceiling, under Ensure's
// 10,000,000 one. This mirrors the cluster-run finding at the level requestbudget operates
// on (cost units, not RSS): 8 policies each individually under the per-program limit,
// refused on the background/reports path and not on the admission path.
const eightExpressionBlobSize = 1_000_000

// TestRequestBudgetScaledForSmallerControllers reproduces the cluster-run finding: 8
// costly-but-under-per-program-limit expressions OOM-killed the background/reports
// controllers (128Mi chart memory limit) into a crash loop while leaving the admission
// controller (384Mi) untouched, because defaultBudget's ~100 MB ceiling is calibrated
// against 384Mi. EnsureBackground scales the ceiling down to match.
func TestRequestBudgetScaledForSmallerControllers(t *testing.T) {
	t.Run("admission budget survives N=8", func(t *testing.T) {
		ctx, _ := requestbudget.Ensure(context.Background())
		_, refusedAt, err := evalNearLimitVariablesSized(t, ctx, 8, eightExpressionBlobSize)
		if refusedAt != 0 {
			t.Fatalf("expected the admission-sized budget to admit all 8 (matches the cluster run: the admission controller correctly refused nothing at N=8), refused at %d: %v", refusedAt, err)
		}
	})
	t.Run("background budget refuses N=8", func(t *testing.T) {
		ctx, _ := requestbudget.EnsureBackground(context.Background())
		_, refusedAt, err := evalNearLimitVariablesSized(t, ctx, 8, eightExpressionBlobSize)
		if refusedAt == 0 {
			t.Fatalf("expected the background-sized budget to refuse before all 8 completed (this is the shape that OOM-killed a 128Mi controller unfixed), all 8 succeeded")
		}
		if !IsResourceError(err) {
			t.Fatalf("expected a resource error, got: %v", err)
		}
	})
}

// TestRequestBudgetBackgroundAdmitsOrdinaryScan is the other half of the finding above: a
// background budget that refuses normal scanning would break reporting for everyone. A
// handful of ordinary, cheap evaluations (well under both the per-program limit and
// EnsureBackground's ceiling) must all still succeed.
func TestRequestBudgetBackgroundAdmitsOrdinaryScan(t *testing.T) {
	ctx, _ := requestbudget.EnsureBackground(context.Background())
	env := regressionEnv(t)
	ast, err := Compile(env, `object.name == "allowed"`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	prg, err := NewProgram(env, ast)
	if err != nil {
		t.Fatalf("program: %v", err)
	}
	for i := 0; i < 100; i++ {
		if _, _, err := prg.ContextEval(ctx, map[string]any{"object": map[string]any{"name": "allowed"}}); err != nil {
			t.Fatalf("ordinary evaluation #%d refused by the background budget: %v", i, err)
		}
	}
}

// TestRequestBudgetPinning fails if the shared request budget is ever removed from
// program.go's ContextEval wrapper: with it wired, 50 near-per-program-limit evaluations on
// one shared ctx must be refused before completing; remove requestbudget.Charge from
// program.go and this test fails (verified by hand while writing it).
func TestRequestBudgetPinning(t *testing.T) {
	budgeted, _ := requestbudget.Ensure(context.Background())
	_, refusedAt, err := evalNearLimitVariables(t, budgeted, 50)
	if refusedAt == 0 {
		t.Fatalf("expected the shared request budget to refuse before 50 near-per-program-limit evaluations completed on one ctx; got no refusal -- program.go's ContextEval no longer charges requestbudget")
	}
	if !IsResourceError(err) {
		t.Fatalf("expected a resource error, got: %v", err)
	}
}
