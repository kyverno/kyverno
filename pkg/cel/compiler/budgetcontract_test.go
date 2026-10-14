package compiler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"gotest.tools/v3/assert"
)

// ContextEval returns a usable value AND the refusal when the per-request budget is
// exhausted by a program that otherwise ran to completion: the charge happens after
// evaluation, so there is a real result in hand when the budget is found to be spent.
//
// This is the contract every caller has to respect. A caller that checks the value before
// the error -- `if out != nil { return out }` -- silently discards the refusal, which is
// exactly what the four families' lazy-variable closures used to do
// (vpol/dpol/mpol/gpol compiler/policy.go). Keep this test next to that invariant: if the
// shape ever changes so that a refusal comes back with a nil value, the ordering in those
// closures stops mattering and this test should be what tells you.
func TestBudgetRefusalArrivesWithAUsableValue(t *testing.T) {
	env, err := NewEnv(cel.Variable("object", cel.DynType))
	assert.NilError(t, err)
	ast, errs := Compile(env, `(object.blob + object.blob).size()`)
	assert.Assert(t, errs == nil, "compile: %v", errs)
	prog, err := NewProgram(env, ast)
	assert.NilError(t, err)

	ctx, release := requestbudget.EnsureBackground(context.Background())
	defer release()

	// Each evaluation is individually legal (well under the per-program limit); only their
	// total exceeds the background ceiling. The blob is sized to keep that true: the
	// expression concatenates it and then calls size() on the result, and size() on a string
	// is charged for its scan, so roughly 0.6 units land per blob byte. At 1,000,000 bytes
	// one evaluation costs about 600,000 of the 1,000,000 per-program limit and the
	// 3,000,000 background ceiling is reached on the sixth. Raise the blob much past this
	// and the per-program limit refuses a single evaluation first, which tests a different
	// limit than the one this test is about.
	data := map[string]any{"object": map[string]any{"blob": strings.Repeat("b", 1_000_000)}}
	for i := 1; i <= 20; i++ {
		val, _, evalErr := prog.ContextEval(ctx, data)
		if evalErr == nil {
			assert.Assert(t, val != nil, "a successful evaluation must return a value")
			continue
		}
		assert.Assert(t, errors.Is(evalErr, requestbudget.ErrBudgetExceeded), "expected a budget refusal, got %v", evalErr)
		assert.Assert(t, IsResourceError(evalErr), "a budget refusal must classify as a resource error")
		assert.Assert(t, val != nil,
			"the refusal must arrive WITH a usable value -- this is why callers must check the error first (got nil value, see this test's comment)")
		return
	}
	t.Fatal("the shared budget never refused; the premise of this test no longer holds")
}
