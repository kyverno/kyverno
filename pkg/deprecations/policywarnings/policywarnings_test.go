package policywarnings

import (
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

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
						FailureAction: ptr.To(kyvernov1.ValidationFailureAction("enforce")),
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

// TestPolicyFieldWarningsValidActions is the negative half of the table: the canonical Enforce
// and Audit values must produce no warnings. Without it the suite would pass even if valid
// actions were also flagged as deprecated.
func TestPolicyFieldWarningsValidActions(t *testing.T) {
	t.Parallel()
	policy := &kyvernov1.ClusterPolicy{
		TypeMeta: metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: "Enforce",
			ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{
				{Action: "Audit"},
			},
			Rules: []kyvernov1.Rule{
				{
					Name: "check",
					Validation: &kyvernov1.Validation{
						FailureAction: ptr.To(kyvernov1.ValidationFailureAction("Enforce")),
						FailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{
							{Action: "Audit"},
						},
					},
				},
			},
		},
	}

	if warnings := PolicyFieldWarnings(policy); len(warnings) != 0 {
		t.Fatalf("expected no warnings for Enforce/Audit, got %d: %#v", len(warnings), warnings)
	}
}
