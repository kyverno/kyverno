package processor

import (
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/v3/assert"
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

func TestAddEngineResponse_AuditWarnMixedRuleOrdering(t *testing.T) {
	t.Parallel()

	// A policy that mixes failure actions, with the Enforce rule ordered first.
	// --audit-warn must classify a failing rule by that rule's own action, not
	// by the policy-wide action (which resolves to the first explicit one and
	// is therefore ordering-sensitive). https://github.com/kyverno/kyverno/issues/12538
	audit := kyvernov1.Audit
	enforce := kyvernov1.Enforce
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mixed"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{{
				Name:       "enforce-rule",
				Validation: &kyvernov1.Validation{FailureAction: &enforce},
			}, {
				Name:       "audit-rule",
				Validation: &kyvernov1.Validation{FailureAction: &audit},
			}},
		},
	}
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "my-resource",
				"namespace": "default",
			},
		},
	}

	tests := []struct {
		name      string
		auditWarn bool
		rule      engineapi.RuleResponse
		expected  ResultCounts
	}{{
		name:      "failing audit rule counts as warn despite an earlier enforce rule",
		auditWarn: true,
		rule:      *engineapi.RuleFail("audit-rule", engineapi.Validation, "validation failed", nil),
		expected:  ResultCounts{Warn: 1},
	}, {
		name:      "failing enforce rule still counts as fail",
		auditWarn: true,
		rule:      *engineapi.RuleFail("enforce-rule", engineapi.Validation, "validation failed", nil),
		expected:  ResultCounts{Fail: 1},
	}, {
		name:      "failing audit rule counts as fail without --audit-warn",
		auditWarn: false,
		rule:      *engineapi.RuleFail("audit-rule", engineapi.Validation, "validation failed", nil),
		expected:  ResultCounts{Fail: 1},
	}}

	// The mirror case, and the one that fails open: with the Audit rule declared
	// first the policy-wide action resolved to Audit, so under --audit-warn a
	// failing Enforce rule was counted as a warning and the command exited 0.
	reversed := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "reversed"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{{
				Name:       "audit-rule",
				Validation: &kyvernov1.Validation{FailureAction: &audit},
			}, {
				Name:       "enforce-rule",
				Validation: &kyvernov1.Validation{FailureAction: &enforce},
			}},
		},
	}
	tests = append(tests, struct {
		name      string
		auditWarn bool
		rule      engineapi.RuleResponse
		expected  ResultCounts
	}{
		name:      "failing enforce rule is not downgraded by an earlier audit rule",
		auditWarn: true,
		rule:      *engineapi.RuleFail("enforce-rule", engineapi.Validation, "validation failed", nil),
		expected:  ResultCounts{Fail: 1},
	})
	policies := map[string]*kyvernov1.ClusterPolicy{
		"failing enforce rule is not downgraded by an earlier audit rule": reversed,
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rc := &ResultCounts{}
			pol := policy
			if p, ok := policies[tt.name]; ok {
				pol = p
			}
			response := engineapi.NewEngineResponse(
				resource,
				engineapi.NewKyvernoPolicy(pol),
				nil,
			).WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{tt.rule},
			})
			rc.addEngineResponse(tt.auditWarn, response)
			assert.Equal(t, tt.expected.Pass, rc.Pass)
			assert.Equal(t, tt.expected.Fail, rc.Fail)
			assert.Equal(t, tt.expected.Warn, rc.Warn)
			assert.Equal(t, tt.expected.Error, rc.Error)
			assert.Equal(t, tt.expected.Skip, rc.Skip)
		})
	}
}
