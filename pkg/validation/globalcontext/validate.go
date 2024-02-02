package globalcontext

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
)

const (
	disabledGctx = "Global context entry would not be processed until it is enabled."
)

type ValidationOptions struct {
	Enabled bool
}

// Validate checks global context entry is valid
func Validate(ctx context.Context, logger logr.Logger, gctx *kyvernov2alpha1.GlobalContextEntry, opts ValidationOptions) ([]string, error) {
	var warnings []string
	if !opts.Enabled {
		warnings = append(warnings, disabledGctx)
	}
	errs := gctx.Validate()
	return warnings, errs.ToAggregate()
}
