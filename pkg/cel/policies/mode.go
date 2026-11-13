package policies

import (
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// EvaluationModeMatchConditionName is the admission match condition that keeps
	// Kyverno from validating policies and exceptions owned by another controller.
	EvaluationModeMatchConditionName = "kyverno-owned-evaluation-mode"

	// PolicyEvaluationModeExpression matches policies Kyverno compiles. An absent
	// spec.evaluation.mode is the Kubernetes default. Policies that do not have
	// the field (for example GeneratingPolicy) also match.
	PolicyEvaluationModeExpression = "!has(object.spec.evaluation.mode) || object.spec.evaluation.mode == '' || object.spec.evaluation.mode == 'Kubernetes' || object.spec.evaluation.mode == 'JSON'"

	// ExceptionEvaluationModeExpression matches PolicyExceptions Kyverno applies.
	// An absent spec.evaluationMode is the Kubernetes default.
	ExceptionEvaluationModeExpression = "!has(object.spec.evaluationMode) || object.spec.evaluationMode == '' || object.spec.evaluationMode == 'Kubernetes' || object.spec.evaluationMode == 'JSON'"
)

// IsKyvernoEvaluationMode reports whether mode is compiled and enforced by Kyverno.
// The empty mode is the Kubernetes default. Any other value, including modes owned
// by Kyverno Authz, is left for that controller.
func IsKyvernoEvaluationMode(mode string) bool {
	switch mode {
	case "", policieskyvernoio.EvaluationModeKubernetes, policieskyvernoio.EvaluationModeJSON:
		return true
	default:
		return false
	}
}

// ForeignEvaluationModeError is returned when a compiler is asked to compile a
// policy whose evaluation mode belongs to another controller. Callers that own
// admission, status, and reporting skip those policies before compiling.
func ForeignEvaluationModeError(mode string) field.ErrorList {
	return field.ErrorList{
		field.Invalid(field.NewPath("spec", "evaluation", "mode"), mode, "evaluation mode is not processed by Kyverno"),
	}
}

// SelectKyvernoExceptions drops exceptions whose evaluationMode is owned by
// another controller. An empty mode stays, matching the Kubernetes default.
func SelectKyvernoExceptions(exceptions []*policiesv1beta1.PolicyException) []*policiesv1beta1.PolicyException {
	if len(exceptions) == 0 {
		return exceptions
	}
	selected := make([]*policiesv1beta1.PolicyException, 0, len(exceptions))
	for _, exception := range exceptions {
		if exception == nil || !IsKyvernoEvaluationMode(exception.Spec.EvaluationMode) {
			continue
		}
		selected = append(selected, exception)
	}
	return selected
}
