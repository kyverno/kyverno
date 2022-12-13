package exception

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
)

// Validate checks policy exception is valid
func Validate(ctx context.Context, logger logr.Logger, polex *kyvernov2alpha1.PolicyException) error {
	// TODO
	return nil
}
