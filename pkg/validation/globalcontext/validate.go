package globalcontext

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate checks global context entry is valid
func Validate(ctx context.Context, logger logr.Logger, gctx *kyvernov2.GlobalContextEntry) ([]string, error) {
	var warnings []string
	errs := gctx.Validate()

	// gctx.Validate() checks projection structure and JMESPath compilation. Keep CEL
	// compilation here so API types remain independent of the runtime compiler.
	path := field.NewPath("spec", "projections")
	for i, p := range gctx.Spec.Projections {
		if p.Expression == "" {
			continue
		}
		if _, compileErrs := compiler.CompileGlobalContextProjection(path.Index(i).Child("expression"), p.Expression); len(compileErrs) > 0 {
			errs = append(errs, compileErrs...)
		}
	}

	return warnings, errs.ToAggregate()
}
