package policies

import (
	"reflect"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

// IsJSONMutatingPolicy reports whether a MutatingPolicy-like object is evaluated
// against a JSON document rather than a Kubernetes admission request. It is the
// single mode gate shared by policy validation, the Kubernetes compiler and
// reconciler, webhook registration, autogen, native MutatingAdmissionPolicy
// generation, policy status and the report controllers, so these surfaces cannot
// drift apart on what counts as a JSON policy. Nil policies are not JSON policies.
func IsJSONMutatingPolicy(policy policiesv1beta1.MutatingPolicyLike) bool {
	if policy == nil {
		return false
	}
	if value := reflect.ValueOf(policy); value.Kind() == reflect.Pointer && value.IsNil() {
		return false
	}
	spec := policy.GetSpec()
	if spec == nil {
		return false
	}
	return spec.EvaluationMode() == policieskyvernoio.EvaluationModeJSON
}
