package compiler

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"gotest.tools/v3/assert"
)

// Routing every CEL evaluation through one shared per-request counter is a tightening: what
// used to be bounded only per expression now also has to fit a request total. This is the
// negative control for that -- an ordinary ImageValidatingPolicy with many expressions must
// still evaluate to completion, and with room to spare.
//
// The headroom is reported as a number rather than left implicit, so a future change that
// makes expressions dearer or the ceiling tighter shows up as a shrinking margin here
// instead of as a refusal in the field.
func TestOrdinaryPolicyWithManyExpressionsStaysWellWithinBudget(t *testing.T) {
	env, err := NewMatchImageEnv()
	assert.NilError(t, err)

	// A realistic registry allowlist: the CRD sets no maxItems on matchImageReferences, so
	// this many is ordinary, not adversarial.
	var refs []v1beta1.MatchImageReference
	for i := 0; i < 24; i++ {
		refs = append(refs, v1beta1.MatchImageReference{
			Expression: fmt.Sprintf(`ref.startsWith("ghcr.io/team-%d/") || ref.contains("@sha256:") || ref == "docker.io/library/nginx:1.27"`, i),
		})
	}
	matchers, errs := CompileMatchImageReferences(nil, env, refs...)
	assert.Assert(t, len(errs) == 0, "compile: %v", errs)

	const image = "ghcr.io/team-7/payments-api@sha256:" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// One budget for the whole policy, the background ceiling because it is the tighter of
	// the two: if an ordinary policy fits here it fits the admission ceiling too.
	ctx, release := requestbudget.EnsureBackground(context.Background())
	defer release()

	for i, m := range matchers {
		_, err := m.Match(ctx, image)
		assert.NilError(t, err, "matchImageReferences[%d] must still evaluate under a shared budget", i)
	}

	// Measure what that policy actually costs, outside the budget, so the margin is a
	// number. ActualCost is the same figure program.go charges.
	var total uint64
	for _, ref := range refs {
		ast, cerrs := Compile(env, ref.Expression)
		assert.Assert(t, cerrs == nil, "compile: %v", cerrs)
		prog, perr := NewProgram(env, ast)
		assert.NilError(t, perr)
		_, details, eerr := prog.ContextEval(context.Background(), map[string]any{ImageRefKey: image})
		assert.NilError(t, eerr)
		if cost := details.ActualCost(); cost != nil {
			total += *cost
		}
	}

	// backgroundBudget is 3,000,000. Assert a wide margin rather than mere success, so this
	// fails while there is still room to react.
	const backgroundCeiling = 3_000_000
	t.Logf("24-expression policy charged %d of %d units (%.4f%% of the background ceiling)",
		total, backgroundCeiling, float64(total)/backgroundCeiling*100)
	assert.Assert(t, total > 0, "the measurement itself must charge something, or it proves nothing")
	assert.Assert(t, total < backgroundCeiling/10,
		"an ordinary policy must use well under a tenth of the request ceiling, charged %d of %d", total, backgroundCeiling)
}

// Once the shared request budget is spent, a further program must be refused BEFORE it runs,
// not merely charged after. cel-go charges a step only once it returns, so without the
// pre-evaluation check in NewProgram's ContextEval the next program would still execute to
// its per-program limit before the refusal; this pins that it does not.
func TestExhaustedRequestBudgetRefusesBeforeEvaluating(t *testing.T) {
	env, err := NewEnv(cel.Variable("object", cel.DynType))
	assert.NilError(t, err)
	ast, cerr := Compile(env, `object.n`)
	assert.NilError(t, cerr)
	prog, perr := NewProgram(env, ast)
	assert.NilError(t, perr)

	ctx, release := requestbudget.Ensure(context.Background())
	defer release()
	// Spend the whole counter outright (any charge at or above the ceiling exhausts it).
	assert.ErrorContains(t, requestbudget.Charge(ctx, 1<<62), "cost budget exceeded")

	// A program evaluated on the spent budget is refused, and the value it would have
	// produced (object.n == 1) never surfaces, proving it did not run.
	val, _, evalErr := prog.ContextEval(ctx, map[string]any{"object": map[string]any{"n": 1}})
	assert.Assert(t, IsResourceError(evalErr), "want a resource refusal, got: %v", evalErr)
	assert.Assert(t, val == nil, "the program must not have evaluated, got value: %v", val)
}
