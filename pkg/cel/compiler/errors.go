package compiler

import (
	"errors"

	"github.com/google/cel-go/interpreter"
	"github.com/kyverno/kyverno/pkg/cel/libs/gzip"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
)

// ErrCostLimitExceeded and ErrGuardRefused are sentinels this package's cost limit and
// guard decorator wrap their own refusals with (via %w), so IsResourceError can recognize
// them downstream -- in particular from __orValueCompat__ (env.go, D18), which must
// propagate a resource refusal instead of silently absorbing it into orValue's default.
var (
	ErrCostLimitExceeded = errors.New("cel: per-program cost limit exceeded")
	ErrGuardRefused      = errors.New("cel: argument or guard limit exceeded")
)

// IsResourceError reports whether err is a refusal produced by this package's cost limit,
// argument cap, or guard table, as opposed to an ordinary CEL type or evaluation error.
func IsResourceError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrCostLimitExceeded) ||
		errors.Is(err, ErrGuardRefused) ||
		errors.Is(err, gzip.ErrOutputTooLarge) ||
		errors.Is(err, requestbudget.ErrBudgetExceeded) {
		return true
	}
	// Also recognize cel-go's own cost cancellation, not just our wrapped sentinel. Only
	// programs built through NewProgram carry the wrapping; a bare env.Program still gets
	// the limit (it rides on the env's program options) but its refusal arrives unwrapped.
	//
	// This is defence-in-depth, not a live bypass fix. Measured: an unwrapped cost refusal
	// already propagates as an error today, .orValue() included, because the limit fires as
	// a panic inside cel-go's Observe and is recovered at the top of ContextEval -- it never
	// becomes a types.Err and so never reaches __orValueCompat__'s binding, whose
	// *types.Err branch is in fact dead code. What this buys is that any future caller
	// switching on IsResourceError classifies a bare-program cost trip correctly.
	var cancelled interpreter.EvalCancelledError
	return errors.As(err, &cancelled) && cancelled.Cause == interpreter.CostLimitExceeded
}
