package background

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKeepResultForEffectiveRevision(t *testing.T) {
	labelsByName := map[string]string{
		"policy":    "vpol.kyverno.io/policy",
		"binding":   "validatingadmissionpolicybinding.apiserver.io/binding",
		"exception": "polex.kyverno.io/exception",
	}
	expected := map[string]string{
		labelsByName["policy"]:    "e1-2-revision",
		labelsByName["binding"]:   "42",
		labelsByName["exception"]: "12",
	}
	for _, tc := range []struct {
		name       string
		result     openreportsv1alpha1.ReportResult
		actual     map[string]string
		wantRetain bool
	}{
		{
			name: "unchanged result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy",
			},
			actual:     map[string]string{labelsByName["policy"]: "e1-2-revision"},
			wantRetain: true,
		},
		{
			name: "legacy label migration must replace stale result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy",
			},
			actual: map[string]string{labelsByName["policy"]: "27"},
		},
		{
			name: "recreated policy must replace stale result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy",
			},
			actual: map[string]string{labelsByName["policy"]: "e1-2-old-uid"},
		},
		{
			name: "unchanged native policy and binding",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy", Properties: map[string]string{"binding": "binding"},
			},
			actual: map[string]string{
				labelsByName["policy"]:  "e1-2-revision",
				labelsByName["binding"]: "42",
			},
			wantRetain: true,
		},
		{
			name: "changed binding must invalidate result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy", Properties: map[string]string{"binding": "binding"},
			},
			actual: map[string]string{
				labelsByName["policy"]:  "e1-2-revision",
				labelsByName["binding"]: "41",
			},
		},
		{
			name: "changed exception must invalidate result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy", Properties: map[string]string{"exceptions": "exception"},
			},
			actual: map[string]string{
				labelsByName["policy"]:    "e1-2-revision",
				labelsByName["exception"]: "11",
			},
		},
		{
			name: "unknown policy is not safe to keep",
			result: openreportsv1alpha1.ReportResult{
				Policy: "unknown",
			},
			actual: map[string]string{labelsByName["policy"]: "e1-2-revision"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := keepResultForEffectiveRevision(tc.result, labelsByName, expected, tc.actual); got != tc.wantRetain {
				t.Fatalf("retain=%v, want %v", got, tc.wantRetain)
			}
		})
	}
}

func TestCELExceptionRevision_IsStableAndTracksUpdates(t *testing.T) {
	first := &policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "first", UID: "uid-1", ResourceVersion: "10",
	}}
	second := &policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "second", UID: "uid-2", ResourceVersion: "20",
	}}
	if got := celExceptionRevision(nil); got != "" {
		t.Fatalf("empty exception set has revision %q", got)
	}
	original := celExceptionRevision([]*policiesv1beta1.PolicyException{first, second})
	if original == "" {
		t.Fatal("non-empty exception set needs revision")
	}
	if got := celExceptionRevision([]*policiesv1beta1.PolicyException{second, first}); got != original {
		t.Fatal("list reordering changed exception revision")
	}
	changed := first.DeepCopy()
	changed.ResourceVersion = "11"
	if celExceptionRevision([]*policiesv1beta1.PolicyException{changed, second}) == original {
		t.Fatal("exception update must change revision")
	}
	if celExceptionRevision([]*policiesv1beta1.PolicyException{first}) == original {
		t.Fatal("exception deletion must change revision")
	}
	if celExceptionRevision([]*policiesv1beta1.PolicyException{first, second, nil}) != original {
		t.Fatal("nil exception changed revision")
	}
}

func TestPolicyRevisions_RespectExceptionScopeAndTargets(t *testing.T) {
	policy := engineapi.NewNamespacedValidatingPolicy(&policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "workload", Name: "policy"},
	})
	native := engineapi.NewValidatingAdmissionPolicy(&admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", ResourceVersion: "10"},
	})
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "workload", Name: "exception", ResourceVersion: "20"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Kind: policy.GetKind(), Name: policy.GetName()}},
		},
	}
	remote := exception.DeepCopy()
	remote.Namespace = "central"
	wrongKind := exception.DeepCopy()
	wrongKind.Spec.PolicyRefs[0].Kind = "MutatingPolicy"
	wrongName := exception.DeepCopy()
	wrongName.Spec.PolicyRefs[0].Name = "other"
	for _, tc := range []struct {
		name      string
		scope     string
		exception *policiesv1beta1.PolicyException
		want      bool
	}{
		{"same namespace", "*", exception, true},
		{"cross namespace", "*", remote, false},
		{"central namespace", "central", remote, true},
		{"outside central namespace", "central", exception, false},
		{"disabled exceptions", "", exception, false},
		{"wrong policy kind", "*", wrongKind, false},
		{"wrong policy name", "*", wrongName, false},
		{"nil exception", "*", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := &controller{exceptionNamespace: tc.scope}
			policies := []engineapi.GenericPolicy{policy, native}
			revisions := ctrl.policyRevisions(policies, []*policiesv1beta1.PolicyException{tc.exception})
			_, found := revisions[celExceptionPolicyLabel(policy)]
			if found != tc.want {
				t.Fatalf("exception revision present=%v, want %v", found, tc.want)
			}
			if celExceptionPolicyLabel(native) != "" {
				t.Fatal("native policy must not have a CEL exception revision label")
			}
			if policyExceptionRevisionChanged(native, revisions, ctrl.policyRevisions(policies, nil)) {
				t.Fatal("CEL exception must not invalidate native policy")
			}
			if got := policyExceptionRevisionChanged(policy, revisions, ctrl.policyRevisions(policies, nil)); got != tc.want {
				t.Fatalf("CEL policy invalidation=%v, want %v", got, tc.want)
			}
		})
	}
}
