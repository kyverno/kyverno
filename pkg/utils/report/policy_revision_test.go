package report

import (
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyRevision_CELKindsIgnoreNonSemanticMetadata(t *testing.T) {
	tests := []struct {
		name string
		wrap func(metav1.ObjectMeta) engineapi.GenericPolicy
	}{
		{"vpol", func(m metav1.ObjectMeta) engineapi.GenericPolicy {
			return engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{ObjectMeta: m})
		}},
		{"nvpol", func(m metav1.ObjectMeta) engineapi.GenericPolicy {
			return engineapi.NewNamespacedValidatingPolicy(&policiesv1beta1.NamespacedValidatingPolicy{ObjectMeta: m})
		}},
		{"mpol", func(m metav1.ObjectMeta) engineapi.GenericPolicy {
			return engineapi.NewMutatingPolicy(&policiesv1beta1.MutatingPolicy{ObjectMeta: m})
		}},
		{"nmpol", func(m metav1.ObjectMeta) engineapi.GenericPolicy {
			return engineapi.NewNamespacedMutatingPolicy(&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: m})
		}},
		{"ivpol", func(m metav1.ObjectMeta) engineapi.GenericPolicy {
			return engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{ObjectMeta: m})
		}},
		{"nivpol", func(m metav1.ObjectMeta) engineapi.GenericPolicy {
			return engineapi.NewNamespacedImageValidatingPolicy(&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: m})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.wrap(metav1.ObjectMeta{Name: "policy", Generation: 7, ResourceVersion: "100"})
			b := tt.wrap(metav1.ObjectMeta{
				Name: "policy", Generation: 7, ResourceVersion: "101",
				Annotations: map[string]string{"example.com/unrelated": "changed"},
			})
			if PolicyRevision(a) != PolicyRevision(b) {
				t.Fatal("metadata-only change invalidated the effective policy revision")
			}
			if got := PolicyRevision(a); !strings.HasPrefix(got, "e1-") || len(got) > 63 {
				t.Fatalf("invalid versioned label value: %q", got)
			}
			c := tt.wrap(metav1.ObjectMeta{Name: "policy", Generation: 8, ResourceVersion: "102"})
			if PolicyRevision(a) == PolicyRevision(c) {
				t.Fatal("new policy generation did not invalidate evaluation")
			}
		})
	}
}

func TestPolicyRevision_TracksRelevantStatusAndExclusion(t *testing.T) {
	a := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Generation: 3, ResourceVersion: "4"},
	}
	conditions := a.DeepCopy()
	conditions.ResourceVersion = "5"
	conditions.Status.ConditionStatus.Message = "conditions reconciled"
	if EffectivePolicyRevision(a) != EffectivePolicyRevision(conditions) {
		t.Fatal("conditions-only status update invalidated report results")
	}
	generated := a.DeepCopy()
	generated.Status.Generated = true
	if EffectivePolicyRevision(a) == EffectivePolicyRevision(generated) {
		t.Fatal("native admission-policy handoff must invalidate results")
	}
	autogen := a.DeepCopy()
	autogen.Status.Autogen.Configs = map[string]policiesv1beta1.ValidatingPolicyAutogen{"deployments": {}}
	if EffectivePolicyRevision(a) == EffectivePolicyRevision(autogen) {
		t.Fatal("effective autogen target change must invalidate results")
	}
	excluded := a.DeepCopy()
	excluded.Labels = map[string]string{kyverno.LabelExcludeReporting: "true"}
	if EffectivePolicyRevision(a) == EffectivePolicyRevision(excluded) {
		t.Fatal("reporting exclusion state must invalidate results")
	}
	excludedAgain := excluded.DeepCopy()
	excludedAgain.Labels[kyverno.LabelExcludeReporting] = "false"
	if EffectivePolicyRevision(excluded) != EffectivePolicyRevision(excludedAgain) {
		t.Fatal("exclude-reporting selector uses presence rather than label value")
	}
}

func TestPolicyRevision_NativePolicyKeepsResourceVersion(t *testing.T) {
	policy := engineapi.NewValidatingAdmissionPolicy(&admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "native", ResourceVersion: "42"},
	})
	if got := PolicyRevision(policy); got != "42" {
		t.Fatalf("native policy revision changed from resourceVersion: %q", got)
	}
}

func TestSetPolicyLabel_PersistsEffectiveRevision(t *testing.T) {
	policy := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Generation: 2, ResourceVersion: "7"},
	})
	report := &reportsv1.EphemeralReport{}
	SetPolicyLabel(report, policy)
	if got := report.GetLabels()[PolicyLabel(policy)]; got != PolicyRevision(policy) {
		t.Fatalf("report revision label mismatch: got %q want %q", got, PolicyRevision(policy))
	}

	// A metadata-only update must write the same report label, so report
	// equality can skip an unnecessary Kubernetes API update.
	metadataOnly := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "policy", Generation: 2, ResourceVersion: "8",
			Annotations: map[string]string{"example.com/unrelated": "changed"},
		},
	})
	updated := &reportsv1.EphemeralReport{}
	SetPolicyLabel(updated, metadataOnly)
	if report.GetLabels()[PolicyLabel(policy)] != updated.GetLabels()[PolicyLabel(metadataOnly)] {
		t.Fatal("metadata-only update rewrote the report policy revision label")
	}
}
