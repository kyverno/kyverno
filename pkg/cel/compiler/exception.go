package compiler

import (
	"slices"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type Exception struct {
	Exception                   *policiesv1beta1.PolicyException
	MatchConditions             []cel.Program
	MatchConditionASTs          []*cel.Ast
	MatchConditionHasSecretRead []bool
	Validations                 []Validation
	// Now is an optional evaluation clock, used by deterministic cache-expiry
	// tests. Production compilation leaves it nil and uses the API's clock.
	Now func() time.Time
}

// IsExpired rechecks a compiled exception at evaluation time. A cached compiled
// policy may outlive spec.expiresAt without any informer event to recompile it.
// A nil exception is left to the compiler's existing input validation path.
func (e Exception) IsExpired() bool {
	if e.Exception == nil {
		return false
	}
	if e.Now == nil {
		return e.Exception.IsExpired()
	}
	expiresAt := e.Exception.Spec.ExpiresAt
	return expiresAt != nil && e.Now().After(expiresAt.Time)
}

// CompileExceptionsWithValidations compiles each exception's match conditions and compensating
// controls, sorted by namespace/name: informer order is not guaranteed, and the first refusal
// supplies the reported message, so an unsorted list would vary that message between requests.
func CompileExceptionsWithValidations(env *cel.Env, exceptions []*policiesv1beta1.PolicyException, trace bool) ([]Exception, field.ErrorList) {
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
		matchConditions, tracedMatchConditions, secretReads, errs := CompileMatchConditionsWithSecretRead(path.Child("matchConditions"), env, trace, polex.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		exception := Exception{
			Exception:                   polex,
			MatchConditions:             matchConditions,
			MatchConditionHasSecretRead: secretReads,
			Validations:                 make([]Validation, 0, len(polex.Spec.Validations)),
		}
		if trace {
			exception.MatchConditionASTs = make([]*cel.Ast, 0, len(tracedMatchConditions))
			for _, condition := range tracedMatchConditions {
				exception.MatchConditionASTs = append(exception.MatchConditionASTs, condition.AST)
			}
		}
		validationsPath := path.Child("validations")
		for i, rule := range polex.Spec.Validations {
			// Exception controls are not part of the trace; only their match-condition ASTs
			// are retained to redact errors from an untraced Secret read.
			validation, errs := CompileValidation(validationsPath.Index(i), env, rule, false)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			exception.Validations = append(exception.Validations, validation)
		}
		compiled = append(compiled, exception)
	}
	return compiled, allErrs
}
