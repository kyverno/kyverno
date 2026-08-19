// Package deprecations centralizes deprecation notices for the legacy
// kyverno.io policy types, which are deprecated in favor of the
// policies.kyverno.io policy types and scheduled for removal.
package deprecations

import (
	"errors"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ErrDeprecated is returned by the CLI loaders when --warnings-as-errors is set
// and a deprecated API version or field is detected. Commands treat it as a
// fatal error (non-zero exit) rather than skipping the file.
var ErrDeprecated = errors.New("deprecated API version or field detected")

// MigrationGuideURL points to the guide describing how to migrate legacy
// kyverno.io policies to the policies.kyverno.io policy types.
const MigrationGuideURL = "https://kyverno.io/docs/guides/migration-to-cel/"

// deprecatedAPIVersions maps a deprecated kyverno.io <apiVersion> <Kind> to the
// apiVersion that should be used instead. The kube-apiserver already emits
// Warning: headers for these via the CRD deprecatedversion marker; this table
// is used by the CLI and metrics where the apiserver warning is not available.
var deprecatedAPIVersions = map[string]string{
	"kyverno.io/v2beta1 ClusterPolicy":        "kyverno.io/v1",
	"kyverno.io/v2beta1 Policy":               "kyverno.io/v1",
	"kyverno.io/v2beta1 PolicyException":      "kyverno.io/v2",
	"kyverno.io/v2beta1 CleanupPolicy":        "kyverno.io/v2",
	"kyverno.io/v2beta1 ClusterCleanupPolicy": "kyverno.io/v2",
	"kyverno.io/v2alpha1 GlobalContextEntry":  "kyverno.io/v2",
}

// APIVersionWarning returns a deprecation warning for using a deprecated
// apiVersion (e.g. kyverno.io/v2beta1 ClusterPolicy), or an empty string if the
// GVK is not a deprecated apiVersion.
func APIVersionWarning(gvk schema.GroupVersionKind) string {
	key := gvk.GroupVersion().String() + " " + gvk.Kind
	replacement, ok := deprecatedAPIVersions[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s is deprecated and will be removed after 1.19; migrate to %s", key, replacement)
}

// PolicyDeprecation describes a deprecated field detected in a policy spec.
type PolicyDeprecation struct {
	// Field is the JSON path of the deprecated field (used as a metric label).
	Field string
	// Message is the human-readable deprecation notice.
	Message string
}

// CheckPolicy returns deprecation warnings for the use of deprecated fields in a
// policy spec (version-agnostic; the spec is always interpreted as kyvernov1).
func CheckPolicy(spec *kyvernov1.Spec) []PolicyDeprecation {
	if spec == nil {
		return nil
	}
	var warnings []PolicyDeprecation
	add := func(field, message string) {
		warnings = append(warnings, PolicyDeprecation{Field: field, Message: message})
	}
	if spec.ValidationFailureAction != "" {
		add("spec.validationFailureAction", "spec.validationFailureAction is deprecated; use validationFailureAction under the validate rule")
	}
	if spec.FailurePolicy != nil {
		add("spec.failurePolicy", "spec.failurePolicy is deprecated; use webhookConfiguration.failurePolicy")
	}
	if spec.WebhookTimeoutSeconds != nil {
		add("spec.webhookTimeoutSeconds", "spec.webhookTimeoutSeconds is deprecated; use webhookConfiguration.timeoutSeconds")
	}
	if spec.GenerateExistingOnPolicyUpdate != nil {
		add("spec.generateExistingOnPolicyUpdate", "spec.generateExistingOnPolicyUpdate is deprecated; use generateExisting under the generate rule")
	}
	if spec.MutateExistingOnPolicyUpdate {
		add("spec.mutateExistingOnPolicyUpdate", "spec.mutateExistingOnPolicyUpdate is deprecated; use mutateExistingOnPolicyUpdate under the mutate rule")
	}
	for i := range spec.Rules {
		rule := &spec.Rules[i]
		for j := range rule.VerifyImages {
			iv := &rule.VerifyImages[j]
			if iv.Image != "" {
				add(fmt.Sprintf("rules[%d].verifyImages.image", i), fmt.Sprintf("rules[%d].verifyImages.image is deprecated; use imageReferences", i))
			}
			if iv.Key != "" {
				add(fmt.Sprintf("rules[%d].verifyImages.key", i), fmt.Sprintf("rules[%d].verifyImages.key is deprecated; use staticKey", i))
			}
			if iv.Roots != "" {
				add(fmt.Sprintf("rules[%d].verifyImages.roots", i), fmt.Sprintf("rules[%d].verifyImages.roots is deprecated; use keyless", i))
			}
			for k := range iv.Attestations {
				if iv.Attestations[k].PredicateType != "" {
					add(fmt.Sprintf("rules[%d].verifyImages.attestations[%d].predicateType", i, k), fmt.Sprintf("rules[%d].verifyImages.attestations[%d].predicateType is deprecated; use type", i, k))
				}
			}
		}
		if rule.Validation != nil && rule.Validation.DeprecatedAssert != nil {
			add(fmt.Sprintf("rules[%d].validate.assert", i), fmt.Sprintf("rules[%d].validate.assert is deprecated and has no effect since 1.19", i))
		}
	}
	return warnings
}

// replacements maps a legacy kyverno.io kind to its policies.kyverno.io replacement(s).
var replacements = map[string]string{
	"ClusterPolicy":        "ValidatingPolicy, MutatingPolicy, GeneratingPolicy or ImageValidatingPolicy",
	"Policy":               "NamespacedValidatingPolicy, NamespacedMutatingPolicy, NamespacedGeneratingPolicy or NamespacedImageValidatingPolicy",
	"ClusterCleanupPolicy": "DeletingPolicy",
	"CleanupPolicy":        "NamespacedDeletingPolicy",
	"PolicyException":      "PolicyException",
}

// Warning returns a deprecation warning for the given legacy kyverno.io kind,
// or an empty string if the kind is not a legacy policy type.
func Warning(kind string) string {
	replacement, ok := replacements[kind]
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		"%s (kyverno.io) is deprecated and will be removed in a future release; migrate to %s (policies.kyverno.io), see %s",
		kind, replacement, MigrationGuideURL,
	)
}
