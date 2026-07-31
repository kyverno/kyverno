package deprecations

import (
	"strings"
	"testing"
)

func TestWarning(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind        string
		replacement string
	}{
		{"ClusterPolicy", "ValidatingPolicy, MutatingPolicy, GeneratingPolicy or ImageValidatingPolicy"},
		{"Policy", "NamespacedValidatingPolicy, NamespacedMutatingPolicy, NamespacedGeneratingPolicy or NamespacedImageValidatingPolicy"},
		{"ClusterCleanupPolicy", "DeletingPolicy"},
		{"CleanupPolicy", "NamespacedDeletingPolicy"},
		{"PolicyException", "PolicyException (policies.kyverno.io)"},
	}
	for _, tt := range tests {
		warning := Warning(tt.kind)
		if !strings.Contains(warning, tt.kind+" (kyverno.io) is deprecated") {
			t.Errorf("Warning(%q) = %q, expected deprecation notice for the kind", tt.kind, warning)
		}
		if !strings.Contains(warning, tt.replacement) {
			t.Errorf("Warning(%q) = %q, expected replacement %q", tt.kind, warning, tt.replacement)
		}
		if !strings.Contains(warning, MigrationGuideURL) {
			t.Errorf("Warning(%q) = %q, expected migration guide URL", tt.kind, warning)
		}
	}
}

func TestWarningNonLegacyKind(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"ValidatingPolicy", "DeletingPolicy", ""} {
		if warning := Warning(kind); warning != "" {
			t.Errorf("Warning(%q) = %q, expected empty string", kind, warning)
		}
	}
}
