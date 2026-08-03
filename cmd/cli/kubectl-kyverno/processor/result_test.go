package processor

import (
	"errors"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
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

func newValidatingPolicyResponse(actions []admissionregistrationv1.ValidationAction, status engineapi.RuleStatus) engineapi.EngineResponse {
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vpol"},
		Spec: policiesv1alpha1.ValidatingPolicySpec{
			ValidationAction: actions,
		},
	}
	rule := engineapi.NewRuleResponse("test-rule", engineapi.Validation, "test failure", status, nil)
	er := engineapi.EngineResponse{
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{*rule},
		},
	}
	return er.WithPolicy(engineapi.NewValidatingPolicyFromLike(vpol))
}

func TestAddValidatingPolicyResponse_AuditWarnTreatsFailAsWarn(t *testing.T) {
	rc := &ResultCounts{}
	resp := newValidatingPolicyResponse([]admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}, engineapi.RuleStatusFail)

	rc.AddValidatingPolicyResponse(true, resp)

	if rc.Fail != 0 || rc.Warn != 1 {
		t.Errorf("expected fail=0 warn=1, got fail=%d warn=%d", rc.Fail, rc.Warn)
	}
}

func TestAddValidatingPolicyResponse_NoAuditWarnFlagCountsFail(t *testing.T) {
	rc := &ResultCounts{}
	resp := newValidatingPolicyResponse([]admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}, engineapi.RuleStatusFail)

	rc.AddValidatingPolicyResponse(false, resp)

	if rc.Fail != 1 || rc.Warn != 0 {
		t.Errorf("expected fail=1 warn=0, got fail=%d warn=%d", rc.Fail, rc.Warn)
	}
}

func TestAddValidatingPolicyResponse_DenyActionAlwaysFails(t *testing.T) {
	rc := &ResultCounts{}
	resp := newValidatingPolicyResponse([]admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}, engineapi.RuleStatusFail)

	rc.AddValidatingPolicyResponse(true, resp)

	if rc.Fail != 1 || rc.Warn != 0 {
		t.Errorf("expected fail=1 warn=0 (deny should never become warn), got fail=%d warn=%d", rc.Fail, rc.Warn)
	}
}

func TestAddValidatingPolicyResponse_PassCountsPass(t *testing.T) {
	rc := &ResultCounts{}
	resp := newValidatingPolicyResponse([]admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}, engineapi.RuleStatusPass)

	rc.AddValidatingPolicyResponse(true, resp)

	if rc.Pass != 1 {
		t.Errorf("expected pass=1, got pass=%d", rc.Pass)
	}
}
