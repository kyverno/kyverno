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

// CanGenerateNativeMutatingPolicyExpressions rejects Kyverno-only exception variables
// in policy CEL expressions. All native MAP API versions copy these expressions
// without declaring an "exceptions" variable. Keep Kyverno's admission path
// active rather than submitting an invalid or semantically different native MAP.
func CanGenerateNativeMutatingPolicyExpressions(policy *policiesv1beta1.MutatingPolicy) (bool, string) {
	// Kyverno evaluates target matching after resolving the target resource, but
	// Kubernetes MutatingAdmissionPolicy has no equivalent target-selection phase.
	// The native builders omit both fields; generating one would silently change
	// which resources are mutated, even when no exception variables are referenced.
	if len(policy.Spec.TargetMatchConditions) > 0 {
		return false, "targetMatchConditions cannot be represented by a native MutatingAdmissionPolicy"
	}
	if policy.Spec.TargetMatchConstraints != nil {
		return false, "targetMatchConstraints cannot be represented by a native MutatingAdmissionPolicy"
	}
	// The native builders currently copy ResourceRules, but omit the other
	// MatchResources selectors and exclusions. Do not broaden mutation scope.
	if constraints := policy.Spec.MatchConstraints; constraints != nil {
		if constraints.NamespaceSelector != nil ||
			constraints.ObjectSelector != nil ||
			constraints.MatchPolicy != nil ||
			len(constraints.ExcludeResourceRules) > 0 {
			return false, "matchConstraints selectors, matchPolicy and excludeResourceRules cannot be represented by the native MutatingAdmissionPolicy builder"
		}
	}
	// The generated native MAP does not copy Kyverno audit annotations.
	if len(policy.Spec.AuditAnnotations) > 0 {
		return false, "auditAnnotations cannot be represented by the native MutatingAdmissionPolicy builder"
	}
	type expression struct {
		source string
		value  string
	}
	var expressions []expression
	for _, condition := range policy.Spec.MatchConditions {
		expressions = append(expressions, expression{"matchCondition " + condition.Name, condition.Expression})
	}
	for _, variable := range policy.Spec.Variables {
		expressions = append(expressions, expression{"variable " + variable.Name, variable.Expression})
	}
	for i, mutation := range policy.Spec.Mutations {
		if mutation.ApplyConfiguration != nil {
			expressions = append(expressions, expression{fmt.Sprintf("mutation %d applyConfiguration", i), mutation.ApplyConfiguration.Expression})
		}
		if mutation.JSONPatch != nil {
			expressions = append(expressions, expression{fmt.Sprintf("mutation %d jsonPatch", i), mutation.JSONPatch.Expression})
		}
	}

	for _, item := range expressions {
		usesExceptions, err := referencesGlobalExceptions(item.value)
		if err != nil {
			return false, fmt.Sprintf("cannot inspect %s for native MutatingAdmissionPolicy generation: %v", item.source, err)
		}
		if usesExceptions {
			return false, fmt.Sprintf("%s references the Kyverno-only exceptions variable, which a native MutatingAdmissionPolicy cannot evaluate", item.source)
		}
	}
	return true, ""
}

// CanGenerateNativePolicyWithMatchConditions also ensures the generated exception conditions
// fit alongside the policy's own conditions within the Kubernetes API limit.
func CanGenerateNativePolicyWithMatchConditions(exceptions []policiesv1beta1.PolicyException, policyMatchConditionCount int) (bool, string) {
	if ok, reason := CanGenerateNativePolicy(exceptions); !ok {
		return false, reason
	}
	return canGenerateWithMatchConditionLimit("ValidatingAdmissionPolicy", exceptions, policyMatchConditionCount)
}

// CanGenerateNativeMutatingPolicyWithMatchConditions also checks the combined native MAP condition count.
func CanGenerateNativeMutatingPolicyWithMatchConditions(exceptions []policiesv1beta1.PolicyException, policyMatchConditionCount int) (bool, string) {
	if ok, reason := CanGenerateNativeMutatingPolicy(exceptions); !ok {
		return false, reason
	}
	return canGenerateWithMatchConditionLimit("MutatingAdmissionPolicy", exceptions, policyMatchConditionCount)
}

func canGenerateWithMatchConditionLimit(kind string, exceptions []policiesv1beta1.PolicyException, policyMatchConditionCount int) (bool, string) {
	const maxMatchConditions = 64
	combinedCount := policyMatchConditionCount + len(negateExceptionMatchConditions(exceptions))
	if combinedCount > maxMatchConditions {
		return false, fmt.Sprintf(
			"generated %s would contain %d matchConditions; Kubernetes allows at most %d",
			kind, combinedCount, maxMatchConditions,
		)
	}
	return true, ""
}

// CanGenerateNativeValidatingPolicyExpressions checks expressions that need
// exception rewriting and the generated variable limit before creating a VAP.
func CanGenerateNativeValidatingPolicyExpressions(policy *policiesv1beta1.ValidatingPolicy, exceptions []policiesv1beta1.PolicyException) (bool, string) {
	type expression struct {
		source string
		value  string
		requiresVariable bool
	}
	var expressions []expression
	for _, condition := range policy.Spec.MatchConditions {
		expressions = append(expressions, expression{"matchCondition " + condition.Name, condition.Expression, false})
	}
	for _, variable := range policy.Spec.Variables {
		expressions = append(expressions, expression{"variable " + variable.Name, variable.Expression, true})
	}
	for i, validation := range policy.Spec.Validations {
		expressions = append(expressions,
			expression{fmt.Sprintf("validation %d", i), validation.Expression, true},
			expression{fmt.Sprintf("validation %d message", i), validation.MessageExpression, true},
		)
	}
	for _, annotation := range policy.Spec.AuditAnnotations {
		expressions = append(expressions, expression{"auditAnnotation " + annotation.Key, annotation.ValueExpression, true})
	}

	needsImages, needsValues := false, false
	for _, exception := range exceptions {
		needsImages = needsImages || len(exception.Spec.Images) > 0
		needsValues = needsValues || len(exception.Spec.AllowedValues) > 0
	}
	for _, item := range expressions {
		// Only these two exception fields can be represented in generated CEL.
		for field, replacement := range map[string]string{
			"exceptions.allowedImages": "[]",
			"exceptions.allowedValues": "[]",
		} {
			rewritten, err := replaceExpressions(item.value, map[string]string{field: replacement})
			if err != nil {
				return false, fmt.Sprintf("cannot inspect %s for native ValidatingAdmissionPolicy generation: %v", item.source, err)
			}
			if item.requiresVariable && rewritten != item.value {
				if field == "exceptions.allowedImages" {
					needsImages = true
				} else {
					needsValues = true
				}
			}
		}
		rewritten, err := replaceExpressions(item.value, map[string]string{
			"exceptions.allowedImages": "[]",
			"exceptions.allowedValues": "[]",
		})
		if err != nil {
			return false, fmt.Sprintf("cannot inspect %s for native ValidatingAdmissionPolicy generation: %v", item.source, err)
		}
		unsupported, err := referencesGlobalExceptions(rewritten)
		if err != nil {
			return false, fmt.Sprintf("cannot inspect %s for native ValidatingAdmissionPolicy generation: %v", item.source, err)
		}
		if unsupported {
			return false, fmt.Sprintf("%s references an unsupported exceptions expression in native ValidatingAdmissionPolicy generation", item.source)
		}
	}
	const maxVariables = 64
	count := len(policy.Spec.Variables)
	if needsImages {
		count++
	}
	if needsValues {
		count++
	}
	if count > maxVariables {
		return false, fmt.Sprintf("generated ValidatingAdmissionPolicy would contain %d variables; Kubernetes allows at most %d", count, maxVariables)
	}
	return true, ""
}
