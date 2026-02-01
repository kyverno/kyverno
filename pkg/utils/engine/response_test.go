package engine

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Tests for Admission Request Blocking Decisions

func TestIsResponseSuccessful_EmptySlice(t *testing.T) {
	// Empty slice should return true (vacuous truth)
	responses := []engineapi.EngineResponse{}
	result := IsResponseSuccessful(responses)
	assert.True(t, result, "empty responses should be considered successful")
}

func TestIsResponseSuccessful_AllSuccessful(t *testing.T) {
	// All successful responses
	responses := []engineapi.EngineResponse{
		engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{}), nil).
			WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{
					*engineapi.RulePass("rule1", engineapi.Validation, "passed", nil),
				},
			}),
		engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{}), nil).
			WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{
					*engineapi.RulePass("rule2", engineapi.Validation, "passed", nil),
				},
			}),
	}

	result := IsResponseSuccessful(responses)
	assert.True(t, result, "all successful responses should return true")
}

func TestIsResponseSuccessful_OneFailedAmongSuccesses(t *testing.T) {
	// One failed response among successes should return false
	responses := []engineapi.EngineResponse{
		engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{}), nil).
			WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{
					*engineapi.RulePass("rule1", engineapi.Validation, "passed", nil),
				},
			}),
		engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{}), nil).
			WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{
					*engineapi.RuleFail("rule2", engineapi.Validation, "failed", nil),
				},
			}),
	}

	result := IsResponseSuccessful(responses)
	assert.False(t, result, "one failed response should cause overall failure")
}

func TestBlockRequest_FailedWithEnforce_Blocks(t *testing.T) {
	// Failed response + Enforce action = should block
	enforce := kyvernov1.ValidationFailureAction("Enforce")
	policy := &kyvernov1.ClusterPolicy{}
	policy.Spec.ValidationFailureAction = enforce

	er := engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil).
		WithPolicyResponse(engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("test-rule", engineapi.Validation, "failed", nil),
			},
		})

	result := BlockRequest(er, kyvernov1.Fail)
	assert.True(t, result, "failed response with Enforce should block")
}

func TestBlockRequest_FailedWithAudit_Allows(t *testing.T) {
	// Failed response + Audit action = should NOT block
	audit := kyvernov1.ValidationFailureAction("Audit")
	policy := &kyvernov1.ClusterPolicy{}
	policy.Spec.ValidationFailureAction = audit

	er := engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil).
		WithPolicyResponse(engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleFail("test-rule", engineapi.Validation, "failed", nil),
			},
		})

	result := BlockRequest(er, kyvernov1.Fail)
	assert.False(t, result, "failed response with Audit should allow")
}

func TestBlockRequest_ErrorWithFailPolicy_Blocks(t *testing.T) {
	// Error response + Fail policy = should block
	policy := &kyvernov1.ClusterPolicy{}

	er := engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil).
		WithPolicyResponse(engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleError("test-rule", engineapi.Validation, "error occurred", nil, nil),
			},
		})

	result := BlockRequest(er, kyvernov1.Fail)
	assert.True(t, result, "error response with Fail policy should block")
}

func TestBlockRequest_ErrorWithIgnorePolicy_Allows(t *testing.T) {
	// Error response + Ignore policy = should NOT block
	policy := &kyvernov1.ClusterPolicy{}

	er := engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil).
		WithPolicyResponse(engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleError("test-rule", engineapi.Validation, "error occurred", nil, nil),
			},
		})

	result := BlockRequest(er, kyvernov1.Ignore)
	assert.False(t, result, "error response with Ignore policy should allow")
}

func TestBlockRequest_SuccessfulResponse_Allows(t *testing.T) {
	// Successful response regardless of policy = should NOT block
	enforce := kyvernov1.ValidationFailureAction("Enforce")
	policy := &kyvernov1.ClusterPolicy{}
	policy.Spec.ValidationFailureAction = enforce

	er := engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil).
		WithPolicyResponse(engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RulePass("test-rule", engineapi.Validation, "passed", nil),
			},
		})

	result := BlockRequest(er, kyvernov1.Fail)
	assert.False(t, result, "successful response should never block")
}

func TestBlockRequest_SkippedResponse_Allows(t *testing.T) {
	// Skipped response = should NOT block
	policy := &kyvernov1.ClusterPolicy{}

	er := engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil).
		WithPolicyResponse(engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleSkip("test-rule", engineapi.Validation, "skipped", nil),
			},
		})

	result := BlockRequest(er, kyvernov1.Fail)
	assert.False(t, result, "skipped response should not block")
}
