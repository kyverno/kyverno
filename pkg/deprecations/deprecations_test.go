package deprecations

import (
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		if !strings.Contains(warning, "kyverno.io "+tt.kind+" is deprecated") {
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

func TestPolicyFieldWarnings(t *testing.T) {
	t.Parallel()
	policy := &kyvernov1.ClusterPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kyverno.io/v1",
			Kind:       "ClusterPolicy",
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: "enforce",
			ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{
				{Action: "audit"},
			},
			Rules: []kyvernov1.Rule{
				{
					Name: "check",
					Validation: &kyvernov1.Validation{
						FailureAction: ptr(kyvernov1.ValidationFailureAction("enforce")),
						FailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{
							{Action: "audit"},
						},
					},
				},
			},
		},
	}

	warnings := PolicyFieldWarnings(policy)
	if len(warnings) != 4 {
		t.Fatalf("expected 4 field warnings, got %d", len(warnings))
	}
	for _, warning := range warnings {
		if warning.Field == "" {
			t.Fatalf("expected field path in warning: %#v", warning)
		}
		if !strings.Contains(warning.Message, "deprecated") {
			t.Fatalf("expected deprecation message, got %q", warning.Message)
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

func ptr[T any](v T) *T {
	return &v
}
