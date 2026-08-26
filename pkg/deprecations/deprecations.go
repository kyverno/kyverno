// Package deprecations centralizes deprecation notices for the legacy
// kyverno.io policy types, which are deprecated in favor of the
// policies.kyverno.io policy types and scheduled for removal.
package deprecations

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MigrationGuideURL points to the guide describing how to migrate legacy
// kyverno.io policies to the policies.kyverno.io policy types.
const MigrationGuideURL = "https://kyverno.io/docs/guides/migration-to-cel/"

// replacements maps a legacy kyverno.io kind to its policies.kyverno.io replacement(s).
var replacements = map[string]string{
	"ClusterPolicy":        "ValidatingPolicy, MutatingPolicy, GeneratingPolicy or ImageValidatingPolicy",
	"Policy":               "NamespacedValidatingPolicy, NamespacedMutatingPolicy, NamespacedGeneratingPolicy or NamespacedImageValidatingPolicy",
	"ClusterCleanupPolicy": "DeletingPolicy",
	"CleanupPolicy":        "NamespacedDeletingPolicy",
	"PolicyException":      "PolicyException",
}

type DeprecationWarning struct {
	Group   string
	Version string
	Kind    string
	Field   string
	Message string
}

// Warning returns a deprecation warning for the given legacy kyverno.io kind,
// or an empty string if the kind is not a legacy policy type.
func Warning(kind string) string {
	warning, ok := BuildKindWarning("kyverno.io", "", kind)
	if !ok {
		return ""
	}
	return warning.Message
}

func BuildKindWarning(group, version, kind string) (DeprecationWarning, bool) {
	replacement, ok := replacements[kind]
	if !ok {
		return DeprecationWarning{}, false
	}
	apiVersion := group
	if version != "" {
		apiVersion = fmt.Sprintf("%s/%s", group, version)
	}
	return DeprecationWarning{
		Group:   group,
		Version: version,
		Kind:    kind,
		Message: fmt.Sprintf(
			"%s %s is deprecated and will be removed in a future release; migrate to %s (policies.kyverno.io), see %s",
			apiVersion, kind, replacement, MigrationGuideURL,
		),
	}, true
}

// PolicyFieldWarnings returns field-level deprecation warnings for legacy policy fields.
func PolicyFieldWarnings(policy kyvernov1.PolicyInterface) []DeprecationWarning {
	var warnings []DeprecationWarning
	spec := policy.GetSpec()
	seen := map[string]struct{}{}
	add := func(fieldPath string) {
		if _, ok := seen[fieldPath]; ok {
			return
		}
		seen[fieldPath] = struct{}{}
		warnings = append(warnings, DeprecationWarning{
			Group:   "kyverno.io",
			Version: policyVersion(policy),
			Kind:    policy.GetKind(),
			Field:   fieldPath,
			Message: "Validation failure actions enforce/audit are deprecated, use Enforce/Audit instead.",
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
