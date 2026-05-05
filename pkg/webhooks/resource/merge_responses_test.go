package resource

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func createTestEngineResponse(policyName string, rules ...engineapi.RuleResponse) engineapi.EngineResponse {
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
	}
	return engineapi.NewEngineResponse(
		unstructured.Unstructured{},
		engineapi.NewKyvernoPolicy(policy),
		nil,
	).WithPolicyResponse(engineapi.PolicyResponse{
		Rules: rules,
	})
}

func TestMergeEngineResponses_EmptyInputs(t *testing.T) {
	tests := []struct {
		name             string
		auditResponses   []engineapi.EngineResponse
		enforceResponses []engineapi.EngineResponse
		expectedCount    int
	}{
		{
			name:             "both empty",
			auditResponses:   nil,
			enforceResponses: nil,
			expectedCount:    0,
		},
		{
			name:             "both empty slices",
			auditResponses:   []engineapi.EngineResponse{},
			enforceResponses: []engineapi.EngineResponse{},
			expectedCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeEngineResponses(tt.auditResponses, tt.enforceResponses)
			assert.Len(t, result, tt.expectedCount)
		})
	}
}

func TestMergeEngineResponses_AuditOnly(t *testing.T) {
	auditRule := *engineapi.RulePass("audit-rule", engineapi.Validation, "audit passed", nil)
	auditResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy-a", auditRule),
		createTestEngineResponse("policy-b", auditRule),
	}

	result := mergeEngineResponses(auditResponses, nil)

	assert.Len(t, result, 2)
	// Verify policy names are preserved
	policyNames := make(map[string]bool)
	for _, r := range result {
		policyNames[r.Policy().GetName()] = true
	}
	assert.True(t, policyNames["policy-a"])
	assert.True(t, policyNames["policy-b"])
}

func TestMergeEngineResponses_EnforceOnly(t *testing.T) {
	enforceRule := *engineapi.RuleFail("enforce-rule", engineapi.Validation, "enforce failed", nil)
	enforceResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy-x", enforceRule),
		createTestEngineResponse("policy-y", enforceRule),
	}

	result := mergeEngineResponses(nil, enforceResponses)

	assert.Len(t, result, 2)
	// Verify policy names are preserved
	policyNames := make(map[string]bool)
	for _, r := range result {
		policyNames[r.Policy().GetName()] = true
	}
	assert.True(t, policyNames["policy-x"])
	assert.True(t, policyNames["policy-y"])
}

func TestMergeEngineResponses_OverlappingPolicies(t *testing.T) {
	auditRule := *engineapi.RulePass("audit-rule", engineapi.Validation, "audit passed", nil)
	enforceRule := *engineapi.RuleFail("enforce-rule", engineapi.Validation, "enforce failed", nil)

	auditResponses := []engineapi.EngineResponse{
		createTestEngineResponse("shared-policy", auditRule),
		createTestEngineResponse("audit-only-policy", auditRule),
	}
	enforceResponses := []engineapi.EngineResponse{
		createTestEngineResponse("shared-policy", enforceRule),
		createTestEngineResponse("enforce-only-policy", enforceRule),
	}

	result := mergeEngineResponses(auditResponses, enforceResponses)

	// Should have 3 responses: shared-policy (merged), audit-only-policy, enforce-only-policy
	assert.Len(t, result, 3)

	// Find the merged response for shared-policy
	var sharedPolicyResponse *engineapi.EngineResponse
	for i := range result {
		if result[i].Policy().GetName() == "shared-policy" {
			sharedPolicyResponse = &result[i]
			break
		}
	}

	assert.NotNil(t, sharedPolicyResponse, "shared-policy response should exist")
	// Merged response should have rules from both audit and enforce
	assert.Len(t, sharedPolicyResponse.PolicyResponse.Rules, 2)
}

func TestMergeEngineResponses_NoOverlap(t *testing.T) {
	auditRule := *engineapi.RulePass("audit-rule", engineapi.Validation, "audit passed", nil)
	enforceRule := *engineapi.RuleFail("enforce-rule", engineapi.Validation, "enforce failed", nil)

	auditResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy-a", auditRule),
	}
	enforceResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy-b", enforceRule),
	}

	result := mergeEngineResponses(auditResponses, enforceResponses)

	assert.Len(t, result, 2)
	policyNames := make(map[string]bool)
	for _, r := range result {
		policyNames[r.Policy().GetName()] = true
	}
	assert.True(t, policyNames["policy-a"])
	assert.True(t, policyNames["policy-b"])
}

func TestMergeEngineResponses_MultipleRulesPerPolicy(t *testing.T) {
	auditRule1 := *engineapi.RulePass("audit-rule-1", engineapi.Validation, "audit passed 1", nil)
	auditRule2 := *engineapi.RulePass("audit-rule-2", engineapi.Validation, "audit passed 2", nil)
	enforceRule1 := *engineapi.RuleFail("enforce-rule-1", engineapi.Validation, "enforce failed 1", nil)

	auditResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy", auditRule1, auditRule2),
	}
	enforceResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy", enforceRule1),
	}

	result := mergeEngineResponses(auditResponses, enforceResponses)

	assert.Len(t, result, 1)
	// Should have 3 rules: 2 from audit + 1 from enforce
	assert.Len(t, result[0].PolicyResponse.Rules, 3)
}

func TestMergeEngineResponses_PreservesRuleStatus(t *testing.T) {
	passRule := *engineapi.RulePass("pass-rule", engineapi.Validation, "passed", nil)
	failRule := *engineapi.RuleFail("fail-rule", engineapi.Validation, "failed", nil)
	skipRule := *engineapi.RuleSkip("skip-rule", engineapi.Validation, "skipped", nil)
	errorRule := *engineapi.RuleError("error-rule", engineapi.Validation, "error", nil, nil)

	auditResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy", passRule, skipRule),
	}
	enforceResponses := []engineapi.EngineResponse{
		createTestEngineResponse("policy", failRule, errorRule),
	}

	result := mergeEngineResponses(auditResponses, enforceResponses)

	assert.Len(t, result, 1)
	rules := result[0].PolicyResponse.Rules
	assert.Len(t, rules, 4)

	// Verify each rule status is preserved
	statusMap := make(map[string]engineapi.RuleStatus)
	for _, rule := range rules {
		statusMap[rule.Name()] = rule.Status()
	}
	assert.Equal(t, engineapi.RuleStatusPass, statusMap["pass-rule"])
	assert.Equal(t, engineapi.RuleStatusFail, statusMap["fail-rule"])
	assert.Equal(t, engineapi.RuleStatusSkip, statusMap["skip-rule"])
	assert.Equal(t, engineapi.RuleStatusError, statusMap["error-rule"])
}
