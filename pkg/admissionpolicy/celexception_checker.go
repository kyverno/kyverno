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

// CanGenerateNativeValidatingPolicy reports whether a ValidatingPolicy still translates to a
// ValidatingAdmissionPolicy. Native matchConditions cannot evaluate exception variables, and
// scoped fine-grained exceptions cannot be represented as static native VAP variables without
// cross-scope evaluation bypass.
func CanGenerateNativeValidatingPolicy(vpol *policiesv1beta1.ValidatingPolicy, exceptions []policiesv1beta1.PolicyException) (bool, string) {
	if vpol != nil {
		for _, mc := range vpol.Spec.MatchConditions {
			if hasExceptionRef(mc.Expression, "") {
				return false, "policy spec.matchConditions reference exception variables (allowedImages/allowedValues), which cannot be evaluated in native ValidatingAdmissionPolicy matchConditions"
			}
			if hasVariableRef(mc.Expression) {
				return false, "policy spec.matchConditions reference variables, which cannot be evaluated in native ValidatingAdmissionPolicy matchConditions"
			}
		}
	}
	return CanGenerateNativePolicy(exceptions)
}

// CanGenerateNativePolicy reports whether a ValidatingPolicy still translates to a
// ValidatingAdmissionPolicy. Controls could be folded in as !(match && controls), but a VAP emits
// only the policy's own message, and it cannot admit a resource the policy already passes.
// In addition, combining multi-condition exceptions must not produce a CEL expression exceeding
// the Kubernetes CEL source-size limit (100,000 code points).
// Scoped fine-grained exceptions (matchConditions + images/allowedValues) cannot be safely
// translated because native VAP variables are evaluated cluster-wide across all requests.
func CanGenerateNativePolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var controlOffending []string
	var scopedFineGrainedOffending []string
	var fullExemptionOffending []string
	var sizeOffending []string
	var allImages []string
	var allValues []string

	for _, exception := range exceptions {
		name := cache.MetaObjectToName(&exception).String()
		if len(exception.Spec.Validations) > 0 {
			controlOffending = append(controlOffending, name)
		}
		hasImages := len(exception.Spec.Images) > 0
		hasValues := len(exception.Spec.AllowedValues) > 0
		hasConditions := len(exception.Spec.MatchConditions) > 0

		if hasConditions && (hasImages || hasValues) {
			scopedFineGrainedOffending = append(scopedFineGrainedOffending, name)
		}
		if !hasConditions && !hasImages && !hasValues && len(exception.Spec.Validations) == 0 {
			fullExemptionOffending = append(fullExemptionOffending, name)
		}
		if !hasConditions && hasImages {
			allImages = append(allImages, exception.Spec.Images...)
		}
		if !hasConditions && hasValues {
			allValues = append(allValues, exception.Spec.AllowedValues...)
		}
		if mc := negateMatchConditions(exception.GetNamespace(), exception.GetName(), exception.Spec.MatchConditions); mc != nil {
			if utf8.RuneCountInString(mc.Expression) > MaxCELCodePoints {
				sizeOffending = append(sizeOffending, name)
			}
		}
	}

	var literalSizeOffending []string
	if len(allImages) > 0 {
		if expr, err := celStringListLiteral(allImages); err == nil && utf8.RuneCountInString(expr) > MaxCELCodePoints {
			literalSizeOffending = append(literalSizeOffending, "allowedImages")
		}
	}
	if len(allValues) > 0 {
		if expr, err := celStringListLiteral(allValues); err == nil && utf8.RuneCountInString(expr) > MaxCELCodePoints {
			literalSizeOffending = append(literalSizeOffending, "allowedValues")
		}
	}

	if len(controlOffending) == 0 && len(scopedFineGrainedOffending) == 0 && len(fullExemptionOffending) == 0 && len(sizeOffending) == 0 && len(literalSizeOffending) == 0 {
		return true, ""
	}

	var reasons []string
	if len(controlOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
			strings.Join(controlOffending, ", "),
		))
	}
	if len(scopedFineGrainedOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define fine-grained exceptions (spec.images / spec.allowedValues) scoped with spec.matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate safely",
			strings.Join(scopedFineGrainedOffending, ", "),
		))
	}
	if len(fullExemptionOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define full exemptions without matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate",
			strings.Join(fullExemptionOffending, ", "),
		))
	}
	if len(sizeOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s produce combined matchConditions exceeding the Kubernetes CEL code point limit (%d), which a generated native policy cannot evaluate",
			strings.Join(sizeOffending, ", "),
			MaxCELCodePoints,
		))
	}
	if len(literalSizeOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"combined fine-grained list(s) %s exceed the Kubernetes CEL code point limit (%d), which a generated native policy cannot evaluate",
			strings.Join(literalSizeOffending, ", "),
			MaxCELCodePoints,
		))
	}
	return false, strings.Join(reasons, "; ")
}

// CanGenerateNativeMutatingPolicy reports whether a MutatingPolicy still translates to a
// native MutatingAdmissionPolicy. Generation is ineligible if fine-grained exceptions are defined
// (unsupported in native MAP), or if combining matchConditions exceeds the CEL source-size limit.
func CanGenerateNativeMutatingPolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var fineGrainedOffending []string
	var fullExemptionOffending []string
	var sizeOffending []string

	for _, exception := range exceptions {
		name := cache.MetaObjectToName(&exception).String()
		hasImages := len(exception.Spec.Images) > 0
		hasValues := len(exception.Spec.AllowedValues) > 0
		hasConditions := len(exception.Spec.MatchConditions) > 0

		if hasImages || hasValues {
			fineGrainedOffending = append(fineGrainedOffending, name)
		}
		if !hasConditions && !hasImages && !hasValues && len(exception.Spec.Validations) == 0 {
			fullExemptionOffending = append(fullExemptionOffending, name)
		}
		if mc := negateMatchConditions(exception.GetNamespace(), exception.GetName(), exception.Spec.MatchConditions); mc != nil {
			if utf8.RuneCountInString(mc.Expression) > MaxCELCodePoints {
				sizeOffending = append(sizeOffending, name)
			}
		}
	}

	if len(fineGrainedOffending) == 0 && len(fullExemptionOffending) == 0 && len(sizeOffending) == 0 {
		return true, ""
	}

	var reasons []string
	if len(fineGrainedOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define fine-grained exceptions (spec.images / spec.allowedValues), which a generated MutatingAdmissionPolicy cannot evaluate",
			strings.Join(fineGrainedOffending, ", "),
		))
	}
	if len(fullExemptionOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define full exemptions without matchConditions, which a generated MutatingAdmissionPolicy cannot evaluate",
			strings.Join(fullExemptionOffending, ", "),
		))
	}
	if len(sizeOffending) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s produce combined matchConditions exceeding the Kubernetes CEL code point limit (%d), which a generated MutatingAdmissionPolicy cannot evaluate",
			strings.Join(sizeOffending, ", "),
			MaxCELCodePoints,
		))
	}
	return false, strings.Join(reasons, "; ")
}
