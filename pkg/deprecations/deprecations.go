// Package deprecations centralizes deprecation notices for the legacy
// kyverno.io policy types, which are deprecated in favor of the
// policies.kyverno.io policy types and scheduled for removal.
package deprecations

import "fmt"

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
