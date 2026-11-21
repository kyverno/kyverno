package compiler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/interpreter"
	"gotest.tools/v3/assert"
)

// A cost refusal must be recognized as a resource error whether or not it carries this
// package's sentinel. Only programs built through NewProgram get the wrapping; a program
// built with a bare env.Program still inherits the limit from the env's program options,
// but its refusal arrives as cel-go's own error. Before this was handled, such a refusal
// was unrecognized and __orValueCompat__ absorbed it into orValue's default -- so
// <expensive>.orValue(fallback) evaluated to the fallback instead of refusing.
func TestIsResourceErrorRecognizesUnwrappedCostRefusal(t *testing.T) {
	unwrapped := error(interpreter.EvalCancelledError{
		Cause:   interpreter.CostLimitExceeded,
		Message: "operation cancelled: actual cost limit exceeded",
	})
	assert.Assert(t, IsResourceError(unwrapped), "cel-go's own cost cancellation must be a resource error")
	assert.Assert(t, IsResourceError(ErrCostLimitExceeded), "the wrapped sentinel must still be recognized")

	// Discrimination matters as much as recognition: an ordinary evaluation error and a
	// context cancellation must not be mistaken for a resource refusal, or orValue stops
	// absorbing the type and presence errors it exists to absorb, and a timeout starts
	// reading as an exhaustion.
	assert.Assert(t, !IsResourceError(errors.New("no such key: spec")), "an ordinary eval error is not a resource refusal")
	ctxCancelled := error(interpreter.EvalCancelledError{
		Cause:   interpreter.ContextCancelled,
		Message: "operation interrupted",
	})
	assert.Assert(t, !IsResourceError(ctxCancelled), "a context cancellation is not a resource refusal")
}

// A cost refusal from a bare env.Program must be classified as a resource error, so any
// consumer of IsResourceError treats it the same as one from NewProgram. The bare
// construction is deliberate: it is the shape that loses the sentinel wrapping.
//
// Note what this does NOT assert. The refusal propagates either way -- cel-go recovers the
// cost panic at the top of ContextEval, so it never becomes a types.Err and .orValue()
// never sees it. This test is about correct classification, not about rescuing a swallowed
// refusal.
func TestBareProgramCostRefusalIsClassifiedAsResourceError(t *testing.T) {
	env, err := NewEnv(cel.Variable("object", cel.DynType))
	assert.NilError(t, err)

	ast, errs := Compile(env, `(object.blob + object.blob + object.blob).size() >= 0`)
	assert.Assert(t, errs == nil, "compile: %v", errs)

	// Bare env.Program: inherits the env's cost limit, but gets no sentinel wrapping.
	bare, err := env.Program(ast) //nolint:forbidigo // deliberately bare: the case under test
	assert.NilError(t, err)

	_, _, evalErr := bare.ContextEval(context.Background(), map[string]any{
		"object": map[string]any{"blob": strings.Repeat("A", 6*1024*1024)},
	})
	assert.Assert(t, evalErr != nil, "an expression far over the per-program limit must refuse")
	assert.Assert(t, IsResourceError(evalErr), "a bare-program cost refusal must classify as a resource error, got %v", evalErr)
}
