package policies

import (
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// kyvernoEvaluationModes is the CEL list of the evaluation modes that Kyverno owns.
const kyvernoEvaluationModes = "['', '" + policieskyvernoio.EvaluationModeKubernetes + "', '" + policieskyvernoio.EvaluationModeJSON + "']"

// PolicyEvaluationModeMatchCondition returns an admission webhook match condition that sends a
// policy to Kyverno only when Kyverno owns its spec.evaluation.mode. Objects without that field,
// such as legacy kyverno.io policies, always match.
func PolicyEvaluationModeMatchCondition() admissionregistrationv1.MatchCondition {
	return admissionregistrationv1.MatchCondition{
		Name:       "kyverno-evaluation-mode",
		Expression: "!has(object.spec) || !has(object.spec.evaluation) || !has(object.spec.evaluation.mode) || object.spec.evaluation.mode in " + kyvernoEvaluationModes,
	}
}

// ExceptionEvaluationModeMatchCondition returns an admission webhook match condition that sends a
// policy exception to Kyverno only when Kyverno owns its spec.evaluationMode.
func ExceptionEvaluationModeMatchCondition() admissionregistrationv1.MatchCondition {
	return admissionregistrationv1.MatchCondition{
		Name:       "kyverno-evaluation-mode",
		Expression: "!has(object.spec) || !has(object.spec.evaluationMode) || object.spec.evaluationMode in " + kyvernoEvaluationModes,
	}
}

// IsKyvernoEvaluationMode reports whether Kyverno owns policies and exceptions with the given
// evaluation mode. Other controllers that share the policies.kyverno.io types, such as Kyverno
// Authz, own every other mode, so Kyverno must not compile, report on, or write status for them.
func IsKyvernoEvaluationMode(mode policiesv1beta1.EvaluationMode) bool {
	switch mode {
	case "", policieskyvernoio.EvaluationModeKubernetes, policieskyvernoio.EvaluationModeJSON:
		return true
	default:
		return false
	}
}

// EvaluationModeAware is implemented by the specs of policy kinds that have an evaluation mode.
type EvaluationModeAware interface {
	EvaluationMode() policiesv1beta1.EvaluationMode
}

// IsKyvernoOwned reports whether Kyverno owns a policy with the given spec.
func IsKyvernoOwned(spec EvaluationModeAware) bool {
	return IsKyvernoEvaluationMode(spec.EvaluationMode())
}

// IsKyvernoOwnedException reports whether Kyverno owns the given policy exception.
func IsKyvernoOwnedException(polex *policiesv1beta1.PolicyException) bool {
	return IsKyvernoEvaluationMode(polex.Spec.EvaluationMode)
}
