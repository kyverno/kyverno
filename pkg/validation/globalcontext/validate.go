package globalcontext

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate checks global context entry is valid
func Validate(ctx context.Context, logger logr.Logger, gctx *kyvernov2beta1.GlobalContextEntry) ([]string, error) {
	var warnings []string
	errs := gctx.Validate()

	// gctx.Validate() only checks projection structure (exactly one of jmesPath/expression
	// set, JMESPath compiles); CEL compilation cannot live in api/ (pkg/cel/compiler
	// already imports api/kyverno/v2beta1), so it is checked here, the single webhook path.
	path := field.NewPath("spec", "projections")
	var jmesPathFields []string
	for i, p := range gctx.Spec.Projections {
		if p.JMESPath != "" {
			jmesPathFields = append(jmesPathFields, path.Index(i).Child("jmesPath").String())
		}
		if p.Expression == "" {
			continue
		}
		if _, compileErrs := compiler.CompileGlobalContextProjection(path.Index(i).Child("expression"), p.Expression); len(compileErrs) > 0 {
			errs = append(errs, compileErrs...)
		}
	}
	if len(jmesPathFields) > 0 {
		warnings = append(warnings, deprecations.BuildProjectionWarning(jmesPathFields).Message)
	}

	return warnings, errs.ToAggregate()
}
