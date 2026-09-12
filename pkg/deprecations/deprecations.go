// Package deprecations centralizes deprecation notices for the legacy
// kyverno.io policy types, which are deprecated in favor of the
// policies.kyverno.io policy types and scheduled for removal.
package deprecations

import (
	"errors"
	"fmt"
	"regexp"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var fieldIndexPattern = regexp.MustCompile(`\[\d+\]`)

// MigrationGuideURL points to the guide describing how to migrate legacy
// kyverno.io policies to the policies.kyverno.io policy types.
const MigrationGuideURL = "https://kyverno.io/docs/guides/migration-to-cel/"

// replacements maps a legacy kyverno.io kind to its policies.kyverno.io replacement(s).
var replacements = map[string]string{
	"ClusterPolicy":        "ValidatingPolicy, MutatingPolicy, GeneratingPolicy or ImageValidatingPolicy",
	"Policy":               "NamespacedValidatingPolicy and the other namespaced policy types",
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
	if group != "kyverno.io" {
		return DeprecationWarning{}, false
	}
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

// IsLegacyPolicyKind reports whether kind (in the given group) is one of the legacy
// kyverno.io policy kinds subject to the 1.20 write-time block on creates/spec-updates.
func IsLegacyPolicyKind(group, kind string) bool {
	if group != "kyverno.io" {
		return false
	}
	_, ok := replacements[kind]
	return ok
}

// legacyPolicyBlockError marks an error returned by BuildKindError, so callers that otherwise treat
// load/parse failures as soft/skippable (e.g. the CLI silently skipping a non-Kyverno YAML file) can
// tell this one apart and surface it as a hard failure instead.
type legacyPolicyBlockError struct{ error }

func (e legacyPolicyBlockError) Unwrap() error { return e.error }

// IsLegacyPolicyBlockError reports whether err is (or wraps) the hard error returned by BuildKindError,
// as opposed to an unrelated YAML/parsing error.
func IsLegacyPolicyBlockError(err error) bool {
	var e legacyPolicyBlockError
	return errors.As(err, &e)
}

// BuildKindError returns a hard error rejecting a create or spec-changing update of a legacy
// kyverno.io policy kind, or false if the kind is not a legacy policy type. It reuses the same
// replacements table and migration URL as BuildKindWarning so the two messages stay consistent.
func BuildKindError(group, version, kind string) (error, bool) {
	if group != "kyverno.io" {
		return nil, false
	}
	replacement, ok := replacements[kind]
	if !ok {
		return nil, false
	}
	apiVersion := group
	if version != "" {
		apiVersion = fmt.Sprintf("%s/%s", group, version)
	}
	return legacyPolicyBlockError{fmt.Errorf(
		"%s %s is no longer accepted for create, or for an update that changes spec; migrate to %s (policies.kyverno.io), see %s",
		apiVersion, kind, replacement, MigrationGuideURL,
	)}, true
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

func NormalizeFieldPath(field string) string {
	return fieldIndexPattern.ReplaceAllString(field, "[]")
}

func policyVersion(policy kyvernov1.PolicyInterface) string {
	if object, ok := policy.(interface{ GetObjectKind() schema.ObjectKind }); ok {
		return object.GetObjectKind().GroupVersionKind().Version
	}
	return ""
}
