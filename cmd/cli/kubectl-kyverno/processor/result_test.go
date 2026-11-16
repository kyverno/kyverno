package processor

import (
	"errors"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAddGenerateResponse_GeneratingPolicy(t *testing.T) {
	t.Parallel()

	gpol := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "example"},
	}
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.io/v1alpha1",
			"kind":       "MyResource",
			"metadata": map[string]interface{}{
				"name":      "my-resource",
				"namespace": "default",
			},
		},
	}

	tests := []struct {
		name     string
		rule     engineapi.RuleResponse
		expected ResultCounts
	}{
		{
			name: "error",
			rule: *engineapi.RuleError("example", engineapi.Generation, "failed to evaluate policy", errors.New("no such key: uid"), nil),
			expected: ResultCounts{
				Error: 1,
			},
		},
		{
			name: "skip",
			rule: *engineapi.RuleSkip("example", engineapi.Generation, "skipped", nil),
			expected: ResultCounts{
				Skip: 1,
			},
		},
		{
			name: "pass",
			rule: *engineapi.RulePass("example", engineapi.Generation, "policy evaluated successfully", nil),
			expected: ResultCounts{
				Pass: 1,
			},
		},
		{
			name: "fail",
			rule: *engineapi.RuleFail("example", engineapi.Generation, "generation failed", nil),
			expected: ResultCounts{
				Fail: 1,
			},
		},
		{
			name: "warn",
			rule: *engineapi.RuleWarn("example", engineapi.Generation, "warning", nil),
			expected: ResultCounts{
				Warn: 1,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rc := &ResultCounts{}
			response := engineapi.NewEngineResponse(
				resource,
				engineapi.NewGeneratingPolicy(gpol),
				nil,
			).WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{tt.rule},
			})
			rc.addGenerateResponse(response)
			assert.Equal(t, tt.expected.Pass, rc.Pass)
			assert.Equal(t, tt.expected.Fail, rc.Fail)
			assert.Equal(t, tt.expected.Warn, rc.Warn)
			assert.Equal(t, tt.expected.Error, rc.Error)
			assert.Equal(t, tt.expected.Skip, rc.Skip)
		})
	}
}

// TestAddValidatingPolicyResponse verifies that AddValidatingPolicyResponse correctly classifies
// pass, fail, warn, and error results, honoring audit-warn for audit-only validating policies.
func TestAddValidatingPolicyResponse(t *testing.T) {
	t.Parallel()

	dummyResource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	tests := []struct {
		name      string
		auditWarn bool
		policy    engineapi.GenericPolicy
		rules     []engineapi.RuleResponse
		expected  ResultCounts
	}{
		{
			name:      "audit-only policy with auditWarn=true counts failure as warn",
			auditWarn: true,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-audit"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Warn: 1},
		},
		{
			name:      "audit-only policy with auditWarn=false counts failure as fail",
			auditWarn: false,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-audit"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "deny policy with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-deny"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "default validationAction (empty) with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-default"},
				Spec:       policiesv1alpha1.ValidatingPolicySpec{},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "explicit empty validationAction slice with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-empty-actions"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "mixed actions (Audit + Deny) with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-mixed"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{
						admissionregistrationv1.Audit,
						admissionregistrationv1.Deny,
					},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "warn-only validationAction with auditWarn=true counts failure as warn",
			auditWarn: true,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-warn-action"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Warn: 1},
		},
		{
			name:      "namespaced validating policy audit-only with auditWarn=true counts failure as warn",
			auditWarn: true,
			policy: engineapi.NewNamespacedValidatingPolicy(&policiesv1beta1.NamespacedValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "nvpol-audit", Namespace: "default"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Warn: 1},
		},
		{
			name:      "image validating policy audit-only with auditWarn=true counts failure as warn",
			auditWarn: true,
			policy: engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "ivpol-audit"},
				Spec: policiesv1alpha1.ImageValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Warn: 1},
		},
		{
			name:      "image validating policy deny with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "ivpol-deny"},
				Spec: policiesv1alpha1.ImageValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "image validating policy audit and deny with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "ivpol-audit-deny"},
				Spec: policiesv1alpha1.ImageValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{
						admissionregistrationv1.Audit,
						admissionregistrationv1.Deny,
					},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "namespaced image validating policy audit-only with auditWarn=true counts failure as warn",
			auditWarn: true,
			policy: engineapi.NewNamespacedImageValidatingPolicy(&policiesv1beta1.NamespacedImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "nivpol-audit", Namespace: "default"},
				Spec: policiesv1alpha1.ImageValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Warn: 1},
		},
		{
			name:      "namespaced image validating policy deny with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewNamespacedImageValidatingPolicy(&policiesv1beta1.NamespacedImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "nivpol-deny", Namespace: "default"},
				Spec: policiesv1alpha1.ImageValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "namespaced image validating policy audit and deny with auditWarn=true counts failure as fail",
			auditWarn: true,
			policy: engineapi.NewNamespacedImageValidatingPolicy(&policiesv1beta1.NamespacedImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "nivpol-audit-deny", Namespace: "default"},
				Spec: policiesv1alpha1.ImageValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{
						admissionregistrationv1.Audit,
						admissionregistrationv1.Deny,
					},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
		{
			name:      "all rule statuses handled correctly",
			auditWarn: true,
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vpol-pass"},
				Spec: policiesv1alpha1.ValidatingPolicySpec{
					ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
				},
			}),
			rules: []engineapi.RuleResponse{
				*engineapi.RulePass("rule-pass", engineapi.Validation, "ok", nil),
				*engineapi.RuleSkip("rule-skip", engineapi.Validation, "skipped", nil),
				*engineapi.RuleError("rule-error", engineapi.Validation, "error", errors.New("err"), nil),
				*engineapi.RuleWarn("rule-warn", engineapi.Validation, "warn", nil),
			},
			expected: ResultCounts{
				Pass:  1,
				Skip:  1,
				Error: 1,
				Warn:  1,
			},
		},
		{
			name:      "nil policy does not panic and counts failure as fail",
			auditWarn: true,
			policy:    nil,
			rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("rule1", engineapi.Validation, "validation failed", nil),
			},
			expected: ResultCounts{Fail: 1},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rc := &ResultCounts{}
			response := engineapi.NewEngineResponse(
				dummyResource,
				tt.policy,
				nil,
			).WithPolicyResponse(engineapi.PolicyResponse{
				Rules: tt.rules,
			})
			rc.AddValidatingPolicyResponse(tt.auditWarn, response)
			assert.Equal(t, tt.expected.Pass, rc.Pass)
			assert.Equal(t, tt.expected.Fail, rc.Fail)
			assert.Equal(t, tt.expected.Warn, rc.Warn)
			assert.Equal(t, tt.expected.Error, rc.Error)
			assert.Equal(t, tt.expected.Skip, rc.Skip)
		})
	}
}
