// Package policywarnings holds the field-level deprecation warnings for legacy kyverno.io
// policies. It lives outside pkg/deprecations because it is the only part of that surface that
// needs the typed legacy API: keeping it here lets pkg/deprecations, which owns the type-free
// write-denial gate, stay compilable after the legacy Go types are deleted. Delete this package
// with its callers when legacy policy validation goes away.
package policywarnings

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PolicyFieldWarnings returns field-level deprecation warnings for legacy policy fields.
func PolicyFieldWarnings(policy kyvernov1.PolicyInterface) []deprecations.DeprecationWarning {
	var warnings []deprecations.DeprecationWarning
	spec := policy.GetSpec()
	seen := map[string]struct{}{}
	add := func(fieldPath string) {
		if _, ok := seen[fieldPath]; ok {
			return
		}
		seen[fieldPath] = struct{}{}
		warnings = append(warnings, deprecations.DeprecationWarning{
			Group:   "kyverno.io",
			Version: policyVersion(policy),
			Kind:    policy.GetKind(),
			Field:   fieldPath,
			Message: fmt.Sprintf("%s: Validation failure actions enforce/audit are deprecated, use Enforce/Audit instead.", fieldPath),
		})
	}

	if isDeprecatedValidationFailureAction(spec.ValidationFailureAction) {
		add("spec.validationFailureAction")
	}
	for i, override := range spec.ValidationFailureActionOverrides {
		if isDeprecatedValidationFailureAction(override.Action) {
			add(fmt.Sprintf("spec.validationFailureActionOverrides[%d].action", i))
		}
	}
	for i, rule := range spec.Rules {
		if rule.Validation != nil && rule.Validation.FailureAction != nil && isDeprecatedValidationFailureAction(*rule.Validation.FailureAction) {
			add(fmt.Sprintf("spec.rules[%d].validate.failureAction", i))
		}
		if rule.Validation != nil {
			for j, override := range rule.Validation.FailureActionOverrides {
				if isDeprecatedValidationFailureAction(override.Action) {
					add(fmt.Sprintf("spec.rules[%d].validate.failureActionOverrides[%d].action", i, j))
				}
			}
		}
	}
	return warnings
}

func isDeprecatedValidationFailureAction(action kyvernov1.ValidationFailureAction) bool {
	return action == "enforce" || action == "audit"
}

func policyVersion(policy kyvernov1.PolicyInterface) string {
	if object, ok := policy.(interface{ GetObjectKind() schema.ObjectKind }); ok {
		return object.GetObjectKind().GroupVersionKind().Version
	}
	return ""
}
