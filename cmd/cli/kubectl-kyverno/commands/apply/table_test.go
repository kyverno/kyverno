package apply

import (
	"bytes"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/trace"
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

// TestSanitizeUserFacingResponses verifies that rule failure messages for Secret resources
// or secret-sensitive evaluations in explain mode are redacted in user-facing table and text output.
func TestSanitizeUserFacingResponses(t *testing.T) {
	vpol := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-secret-data"},
	})

	secretRes := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "my-secret",
			},
		},
	}

	podRes := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "my-pod",
			},
		},
	}

	secretResp := engineapi.EngineResponse{
		Resource: secretRes,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("check-data", engineapi.Validation, "sensitive password was: s3cr3t", nil),
			},
		},
	}.WithPolicy(vpol)

	podResp := engineapi.EngineResponse{
		Resource: podRes,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("check-pod", engineapi.Validation, "normal error: image tag latest is forbidden", nil),
			},
		},
	}.WithPolicy(vpol)

	cfg := &ApplyCommandConfig{
		Explain: false,
	}

	// Secret resource rule messages must be redacted even when explain is false
	sanitized := cfg.sanitizeUserFacingResponses([]engineapi.EngineResponse{secretResp, podResp})
	assert.Equal(t, "(redacted)", sanitized[0].PolicyResponse.Rules[0].Message())
	assert.Equal(t, "normal error: image tag latest is forbidden", sanitized[1].PolicyResponse.Rules[0].Message())

	// Test printTable receives redacted message in detailed mode
	var buf bytes.Buffer
	printTable(&buf, true, false, sanitized...)
	tableOut := buf.String()
	assert.Contains(t, tableOut, "(redacted)")
	assert.NotContains(t, tableOut, "s3cr3t")
	assert.Contains(t, tableOut, "normal error: image tag latest is forbidden")
}

func TestSanitizeUserFacingResponses_ExplainDynamicMessage(t *testing.T) {
	vpol := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-secret-expr"},
	})

	podRes := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "my-pod",
			},
		},
	}

	podResp := engineapi.EngineResponse{
		Resource: podRes,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("check-pod", engineapi.Validation, "message expression evaluated to: super-secret-token", nil),
			},
		},
	}.WithPolicy(vpol)

	cfg := &ApplyCommandConfig{
		Explain: true,
	}

	// Decision with secret reads and dynamic message
	cfg.recordDecision(&trace.Decision{
		PolicyKind:   "ValidatingPolicy",
		PolicyName:   "check-secret-expr",
		ResourceKind: "Pod",
		ResourceName: "my-pod",
		Variables: []trace.NamedExpressionTrace{
			{
				Name: "secretVar",
				ExpressionTrace: trace.ExpressionTrace{
					Inputs: trace.Inputs{Secret: true},
				},
			},
		},
		Verdict: trace.VerdictTrace{
			Status:         trace.VerdictFail,
			DynamicMessage: true,
		},
	})

	sanitized := cfg.sanitizeUserFacingResponses([]engineapi.EngineResponse{podResp})
	assert.Equal(t, "(redacted)", sanitized[0].PolicyResponse.Rules[0].Message())

	var buf bytes.Buffer
	printTable(&buf, true, false, sanitized...)
	assert.Contains(t, buf.String(), "(redacted)")
	assert.NotContains(t, buf.String(), "super-secret-token")
}

func TestSanitizeUserFacingResponses_PolicyKindDisambiguation(t *testing.T) {
	vpol := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check"},
	})
	ivpol := engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check"},
	})

	podRes := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "my-pod",
			},
		},
	}

	vpolResp := engineapi.EngineResponse{
		Resource: podRes,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("vpol-rule", engineapi.Validation, "sensitive dynamic value: secret123", nil),
			},
		},
	}.WithPolicy(vpol)

	ivpolResp := engineapi.EngineResponse{
		Resource: podRes,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("ivpol-rule", engineapi.ImageVerify, "static failure: image tag is missing", nil),
			},
		},
	}.WithPolicy(ivpol)

	cfg := &ApplyCommandConfig{
		Explain: true,
	}

	// Record secret decision for ValidatingPolicy "check"
	cfg.recordDecision(&trace.Decision{
		PolicyKind:   "ValidatingPolicy",
		PolicyName:   "check",
		ResourceKind: "Pod",
		ResourceName: "my-pod",
		Verdict: trace.VerdictTrace{
			Status:         trace.VerdictFail,
			DynamicMessage: true,
		},
	})

	// Record non-secret decision for ImageValidatingPolicy "check"
	cfg.recordDecision(&trace.Decision{
		PolicyKind:   "ImageValidatingPolicy",
		PolicyName:   "check",
		ResourceKind: "Pod",
		ResourceName: "my-pod",
		Verdict: trace.VerdictTrace{
			Status:         trace.VerdictFail,
			DynamicMessage: false,
		},
	})

	sanitized := cfg.sanitizeUserFacingResponses([]engineapi.EngineResponse{vpolResp, ivpolResp})
	// ValidatingPolicy rule was dynamic secret -> redacted
	assert.Equal(t, "(redacted)", sanitized[0].PolicyResponse.Rules[0].Message())
	// ImageValidatingPolicy rule was non-secret static -> preserved
	assert.Equal(t, "static failure: image tag is missing", sanitized[1].PolicyResponse.Rules[0].Message())
}
