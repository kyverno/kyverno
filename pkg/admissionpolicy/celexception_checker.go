package admissionpolicy

import (
	"fmt"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/client-go/tools/cache"
)

// CanGenerateNativePolicy reports whether a ValidatingPolicy still translates to a
// ValidatingAdmissionPolicy. A VAP cannot represent compensating exception validations or
// conditionally scope image/value exceptions: its match conditions skip the whole policy, while
// the CEL engine keeps evaluating the policy and exposes scoped values only when the exception matches.
func CanGenerateNativePolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var reasons []string
	var offendingValidations []string
	var offendingScopedConditions []string
	for _, exception := range exceptions {
		name := cache.MetaObjectToName(&exception).String()
		if len(exception.Spec.Validations) > 0 {
			offendingValidations = append(offendingValidations, name)
		}
		if len(exception.Spec.MatchConditions) > 0 &&
			(len(exception.Spec.Images) > 0 || len(exception.Spec.AllowedValues) > 0) {
			offendingScopedConditions = append(offendingScopedConditions, name)
		}
	}
	if len(offendingValidations) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
			strings.Join(offendingValidations, ", "),
		))
	}
	if len(offendingScopedConditions) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s combine matchConditions with image/value scopes, which a generated ValidatingAdmissionPolicy cannot evaluate",
			strings.Join(offendingScopedConditions, ", "),
		))
	}
	if len(reasons) > 0 {
		return false, strings.Join(reasons, "; ")
	}
	return true, ""
}


// CanGenerateNativeMutatingPolicy reports whether the CEL exceptions can be represented
// by a native MutatingAdmissionPolicy. The current MAP builder cannot expose conditional
// image/value scopes or apply PolicyException compensating validations.
func CanGenerateNativeMutatingPolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var offending []string
	for _, exception := range exceptions {
		if len(exception.Spec.Images) > 0 ||
			len(exception.Spec.AllowedValues) > 0 ||
			len(exception.Spec.Validations) > 0 {
			offending = append(offending, cache.MetaObjectToName(&exception).String())
		}
	}
	if len(offending) == 0 {
		return true, ""
	}
	return false, fmt.Sprintf(
		"policy exception(s) %s use image/value scopes or compensating controls, which a generated MutatingAdmissionPolicy cannot evaluate",
		strings.Join(offending, ", "),
	)
}
