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
		{"Policy", "NamespacedValidatingPolicy and the other namespaced policy types"},
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

// TestWarningLength ensures kind deprecation messages stay within the 256
// character limit Kubernetes enforces for CRD .spec.versions[].deprecationWarning,
// so the same wording can be reused in kubebuilder deprecatedversion markers.
func TestWarningLength(t *testing.T) {
	t.Parallel()
	for kind := range replacements {
		for _, version := range []string{"v1", "v2", "v2beta1"} {
			warning, ok := BuildKindWarning("kyverno.io", version, kind)
			if !ok {
				t.Fatalf("expected warning for kind %q", kind)
			}
			if len(warning.Message) > 256 {
				t.Errorf("warning for %s/%s %s is %d characters, exceeding the 256 character CRD deprecationWarning limit: %q",
					"kyverno.io", version, kind, len(warning.Message), warning.Message)
			}
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

func TestBuildKindWarningIgnoresNonKyvernoGroup(t *testing.T) {
	t.Parallel()
	if _, ok := BuildKindWarning("policies.kyverno.io", "v1", "PolicyException"); ok {
		t.Fatalf("expected non-kyverno.io group to be ignored")
	}
}

func TestIsLegacyPolicyKind(t *testing.T) {
	t.Parallel()
	for kind := range replacements {
		if !IsLegacyPolicyKind("kyverno.io", kind) {
			t.Errorf("IsLegacyPolicyKind(%q, %q) = false, want true", "kyverno.io", kind)
		}
	}
	for _, tt := range []struct{ group, kind string }{
		{"policies.kyverno.io", "PolicyException"},
		{"policies.kyverno.io", "ValidatingPolicy"},
		{"kyverno.io", "ValidatingPolicy"},
		{"kyverno.io", ""},
	} {
		if IsLegacyPolicyKind(tt.group, tt.kind) {
			t.Errorf("IsLegacyPolicyKind(%q, %q) = true, want false", tt.group, tt.kind)
		}
	}
}

func TestBuildKindError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind        string
		replacement string
	}{
		{"ClusterPolicy", "ValidatingPolicy, MutatingPolicy, GeneratingPolicy or ImageValidatingPolicy"},
		{"Policy", "NamespacedValidatingPolicy and the other namespaced policy types"},
		{"ClusterCleanupPolicy", "DeletingPolicy"},
		{"CleanupPolicy", "NamespacedDeletingPolicy"},
		{"PolicyException", "PolicyException (policies.kyverno.io)"},
	}
	for _, tt := range tests {
		err, ok := BuildKindError("kyverno.io", "v1", tt.kind)
		if !ok {
			t.Fatalf("BuildKindError(%q) ok = false, want true", tt.kind)
		}
		if err == nil {
			t.Fatalf("BuildKindError(%q) returned a nil error", tt.kind)
		}
		msg := err.Error()
		if !strings.Contains(msg, "kyverno.io/v1 "+tt.kind+" is no longer accepted for create, or for an update that changes spec") {
			t.Errorf("BuildKindError(%q) = %q, expected a rejection notice for the kind", tt.kind, msg)
		}
		if !strings.Contains(msg, tt.replacement) {
			t.Errorf("BuildKindError(%q) = %q, expected replacement %q", tt.kind, msg, tt.replacement)
		}
		if !strings.Contains(msg, MigrationGuideURL) {
			t.Errorf("BuildKindError(%q) = %q, expected migration guide URL", tt.kind, msg)
		}
	}
}

func TestBuildKindErrorNonLegacyKind(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"ValidatingPolicy", "DeletingPolicy", ""} {
		if err, ok := BuildKindError("kyverno.io", "v1", kind); ok || err != nil {
			t.Errorf("BuildKindError(%q) = (%v, %v), expected (nil, false)", kind, err, ok)
		}
	}
}

func TestBuildKindErrorIgnoresNonKyvernoGroup(t *testing.T) {
	t.Parallel()
	if _, ok := BuildKindError("policies.kyverno.io", "v1", "PolicyException"); ok {
		t.Fatalf("expected non-kyverno.io group to be ignored")
	}
}

func TestNormalizeFieldPath(t *testing.T) {
	t.Parallel()
	got := NormalizeFieldPath("spec.rules[3].validate.failureActionOverrides[7].action")
	want := "spec.rules[].validate.failureActionOverrides[].action"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func ptr[T any](v T) *T {
	return &v
}
