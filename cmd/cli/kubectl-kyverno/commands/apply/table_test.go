package apply

import (
	"bytes"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestPrintTable_AuditWarn verifies that printTable displays audit-only validating policy
// failures as warnings in table output when auditWarn is enabled.
func TestPrintTable_AuditWarn(t *testing.T) {
	vpolAudit := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-audit"},
		Spec: policiesv1alpha1.ValidatingPolicySpec{
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
		},
	})

	vpolDeny := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-deny"},
		Spec: policiesv1alpha1.ValidatingPolicySpec{
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	})

	res := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		},
	}

	auditResp := engineapi.EngineResponse{
		Resource: res,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule-fail", engineapi.Validation, "failed check", nil),
			},
		},
	}.WithPolicy(vpolAudit)

	denyResp := engineapi.EngineResponse{
		Resource: res,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule-fail", engineapi.Validation, "failed check", nil),
			},
		},
	}.WithPolicy(vpolDeny)

	t.Run("auditWarn false shows Fail for audit policy", func(t *testing.T) {
		var buf bytes.Buffer
		printTable(&buf, false, false, auditResp)
		out := buf.String()
		assert.Contains(t, out, "Fail")
		assert.NotContains(t, out, "Warn")
	})

	t.Run("auditWarn true shows Warn for audit policy", func(t *testing.T) {
		var buf bytes.Buffer
		printTable(&buf, false, true, auditResp)
		out := buf.String()
		assert.Contains(t, out, "Warn")
		assert.NotContains(t, out, "Fail")
	})

	t.Run("auditWarn true shows Fail for deny policy", func(t *testing.T) {
		var buf bytes.Buffer
		printTable(&buf, false, true, denyResp)
		out := buf.String()
		assert.Contains(t, out, "Fail")
		assert.NotContains(t, out, "Warn")
	})
}
