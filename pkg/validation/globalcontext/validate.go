package globalcontext

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

// Validate checks global context entry is valid
func Validate(ctx context.Context, logger logr.Logger, gctx *kyvernov2beta1.GlobalContextEntry) ([]string, error) {
	var warnings []string
	errs := gctx.Validate()
	return warnings, errs.ToAggregate()
}
