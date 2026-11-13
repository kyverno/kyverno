package admissionpolicy

import (
	"fmt"
	"strings"
	"unicode/utf8"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/client-go/tools/cache"
)

// MaxCELCodePoints is the maximum number of Unicode code points allowed in a Kubernetes CEL expression (100,000).
const MaxCELCodePoints = 100000

// CanGenerateNativePolicy reports whether a ValidatingPolicy still translates to a
// ValidatingAdmissionPolicy. Controls could be folded in as !(match && controls), but a VAP emits
// only the policy's own message, and it cannot admit a resource the policy already passes.
// In addition, combining multi-condition exceptions must not produce a CEL expression exceeding
// the Kubernetes CEL source-size limit (100,000 code points).
func CanGenerateNativePolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var controlOffending []string
	var sizeOffending []string
	for _, exception := range exceptions {
		name := cache.MetaObjectToName(&exception).String()
		if len(exception.Spec.Validations) > 0 {
			controlOffending = append(controlOffending, name)
		}
		if mc := negateMatchConditions(exception.GetNamespace(), exception.GetName(), exception.Spec.MatchConditions); mc != nil {
			if utf8.RuneCountInString(mc.Expression) > MaxCELCodePoints {
				sizeOffending = append(sizeOffending, name)
			}
		}
	}
	if len(controlOffending) == 0 && len(sizeOffending) == 0 {
		return true, ""
	}
	var reasons []string
	if len(controlOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
			strings.Join(controlOffending, ", "),
		))
	}
	if len(sizeOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s produce combined matchConditions exceeding the Kubernetes CEL code point limit (%d), which a generated native policy cannot evaluate",
			strings.Join(sizeOffending, ", "),
			MaxCELCodePoints,
		))
	}
	return false, strings.Join(reasons, "; ")
}

// CanGenerateNativeMutatingPolicy reports whether a MutatingPolicy still translates to a
// native MutatingAdmissionPolicy. Generation is ineligible if combining an exception's
// matchConditions produces a CEL expression exceeding the Kubernetes source-size limit (100,000 code points).
// Unlike ValidatingAdmissionPolicies, MutatingAdmissionPolicies perform a size-only check and do not
// reject exceptions with compensating controls (spec.validations), which are validating-only.
func CanGenerateNativeMutatingPolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var sizeOffending []string
	for _, exception := range exceptions {
		if mc := negateMatchConditions(exception.GetNamespace(), exception.GetName(), exception.Spec.MatchConditions); mc != nil {
			if utf8.RuneCountInString(mc.Expression) > MaxCELCodePoints {
				sizeOffending = append(sizeOffending, cache.MetaObjectToName(&exception).String())
			}
		}
	}
	if len(sizeOffending) == 0 {
		return true, ""
	}
	return false, fmt.Sprintf(
		"policy exception(s) %s produce combined matchConditions exceeding the Kubernetes CEL code point limit (%d), which a generated MutatingAdmissionPolicy cannot evaluate",
		strings.Join(sizeOffending, ", "),
		MaxCELCodePoints,
	)
}
