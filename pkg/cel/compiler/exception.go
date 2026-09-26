package compiler

import (
	"slices"
	"strings"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type Exception struct {
	Exception       *policiesv1beta1.PolicyException
	MatchConditions []cel.Program
	Validations     []Validation
}

// CompileExceptionsWithValidations compiles each exception's match conditions and compensating
// controls, sorted by namespace/name: informer order is not guaranteed, and the first refusal
// supplies the reported message, so an unsorted list would vary that message between requests.
func CompileExceptionsWithValidations(env *cel.Env, exceptions []*policiesv1beta1.PolicyException) ([]Exception, field.ErrorList) {
	var allErrs field.ErrorList
	sorted := slices.Clone(exceptions)
	slices.SortFunc(sorted, func(a, b *policiesv1beta1.PolicyException) int {
		if ns := strings.Compare(a.GetNamespace(), b.GetNamespace()); ns != 0 {
			return ns
		}
		return strings.Compare(a.GetName(), b.GetName())
	})
	compiled := make([]Exception, 0, len(sorted))
	for _, polex := range sorted {
		// root the path at the exception, so an error in one is not reported as if it were in the
		// policy's own spec
		path := field.NewPath("exception").Key(polex.GetNamespace() + "/" + polex.GetName()).Child("spec")
		matchConditions, errs := CompileMatchConditions(path.Child("matchConditions"), env, polex.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		exception := Exception{
			Exception:       polex,
			MatchConditions: matchConditions,
			Validations:     make([]Validation, 0, len(polex.Spec.Validations)),
		}
		validationsPath := path.Child("validations")
		for i, rule := range polex.Spec.Validations {
			validation, errs := CompileValidation(validationsPath.Index(i), env, rule)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			exception.Validations = append(exception.Validations, validation)
		}
		compiled = append(compiled, exception)
	}
	return compiled, allErrs
}
