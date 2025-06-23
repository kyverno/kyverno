package globalcontext

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
)

// Validate checks global context entry is valid
func Validate(ctx context.Context, logger logr.Logger, gctx *kyvernov2alpha1.GlobalContextEntry) ([]string, error) {
	var warnings []string
	errs := gctx.Validate()
	return warnings, errs.ToAggregate()
}
