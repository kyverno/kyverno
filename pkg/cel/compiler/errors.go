package compiler

import (
	"errors"

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
	return errors.Is(err, ErrCostLimitExceeded) ||
		errors.Is(err, ErrGuardRefused) ||
		errors.Is(err, gzip.ErrOutputTooLarge) ||
		errors.Is(err, requestbudget.ErrBudgetExceeded)
}
