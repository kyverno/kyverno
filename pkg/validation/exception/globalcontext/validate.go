package globalcontext

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

const (
	disabledGctx = "Global context entry would not be processed until it is enabled."
)

type ValidationOptions struct {
	Enabled bool
}

// Validate checks global context entry is valid
//
// Dead code (nothing in this repository calls it): it calls gctx.Validate() directly and
// does not run the CEL compile loop in pkg/validation/globalcontext.Validate. If this is
// ever revived, route through pkg/validation/globalcontext.Validate instead, or expression
// projections would bypass CEL compilation.
func Validate(ctx context.Context, logger logr.Logger, gctx *kyvernov2beta1.GlobalContextEntry, opts ValidationOptions) ([]string, error) {
	var warnings []string
	if !opts.Enabled {
		warnings = append(warnings, disabledGctx)
	}
	errs := gctx.Validate()
	return warnings, errs.ToAggregate()
}
