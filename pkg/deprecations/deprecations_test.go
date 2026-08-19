package deprecations

import (
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAPIVersionWarning(t *testing.T) {
	t.Parallel()
	tests := []struct {
		gvk       schema.GroupVersionKind
		expectMsg bool
	}{
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterPolicy"}, true},
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "Policy"}, true},
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "PolicyException"}, true},
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "CleanupPolicy"}, true},
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterCleanupPolicy"}, true},
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"}, true},
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}, false},
		{schema.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "PolicyException"}, false},
	}
	for _, tt := range tests {
		warning := APIVersionWarning(tt.gvk)
		if tt.expectMsg && warning == "" {
			t.Errorf("APIVersionWarning(%q) = %q, expected a deprecation warning", tt.gvk, warning)
		}
		if !tt.expectMsg && warning != "" {
			t.Errorf("APIVersionWarning(%q) = %q, expected empty string", tt.gvk, warning)
		}
		if warning != "" && !strings.Contains(warning, "will be removed after 1.19") {
			t.Errorf("APIVersionWarning(%q) = %q, expected removal notice", tt.gvk, warning)
		}
	}
}

func TestCheckPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		spec     *kyvernov1.Spec
		expected []string
	}{
		{
			name:     "nil spec",
			spec:     nil,
			expected: nil,
		},
		{
			name: "spec-level deprecated fields",
			spec: &kyvernov1.Spec{
				ValidationFailureAction:        "Enforce",
				FailurePolicy:                  failurePolicyTypePtr("Fail"),
				WebhookTimeoutSeconds:          int32Ptr(10),
				GenerateExistingOnPolicyUpdate: boolPtr(true),
				MutateExistingOnPolicyUpdate:   true,
			},
			expected: []string{
				"spec.validationFailureAction is deprecated",
				"spec.failurePolicy is deprecated",
				"spec.webhookTimeoutSeconds is deprecated",
				"spec.generateExistingOnPolicyUpdate is deprecated",
				"spec.mutateExistingOnPolicyUpdate is deprecated",
			},
		},
		{
			name: "rule-level deprecated verify image fields",
			spec: &kyvernov1.Spec{
				Rules: []kyvernov1.Rule{
					{
						VerifyImages: []kyvernov1.ImageVerification{
							{Image: "nginx", Key: "key", Roots: "roots", Attestations: []kyvernov1.Attestation{{PredicateType: "custom"}}},
						},
						Validation: &kyvernov1.Validation{DeprecatedAssert: &kyvernov1.Any{}},
					},
				},
			},
			expected: []string{
				"rules[0].verifyImages.image is deprecated",
				"rules[0].verifyImages.key is deprecated",
				"rules[0].verifyImages.roots is deprecated",
				"rules[0].verifyImages.attestations[0].predicateType is deprecated",
				"rules[0].validate.assert is deprecated",
			},
		},
		{
			name: "no deprecated fields",
			spec: &kyvernov1.Spec{
				WebhookConfiguration: &kyvernov1.WebhookConfiguration{TimeoutSeconds: int32Ptr(10)},
				Rules: []kyvernov1.Rule{
					{Validation: &kyvernov1.Validation{FailureAction: validationFailureActionPtr("Enforce")}},
				},
			},
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			warnings := CheckPolicy(tt.spec)
			for _, want := range tt.expected {
				found := false
				for _, got := range warnings {
					if got.Field == "" {
						t.Errorf("CheckPolicy() returned warning with empty Field: %+v", got)
					}
					if strings.Contains(got.Message, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("CheckPolicy() missing warning %q, got %v", want, warnings)
				}
			}
			if len(warnings) != len(tt.expected) {
				t.Errorf("CheckPolicy() got %d warnings %v, expected %d", len(warnings), warnings, len(tt.expected))
			}
		})
	}
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }
func failurePolicyTypePtr(s string) *kyvernov1.FailurePolicyType {
	v := kyvernov1.FailurePolicyType(s)
	return &v
}
func validationFailureActionPtr(s string) *kyvernov1.ValidationFailureAction {
	v := kyvernov1.ValidationFailureAction(s)
	return &v
}

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
