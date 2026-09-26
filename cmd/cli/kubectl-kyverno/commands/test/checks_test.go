package test

import (
	"testing"

	"github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEvaluateCELAssertions(t *testing.T) {
	activation := map[string]any{
		"result": map[string]any{
			"status":  "fail",
			"message": "The label `team` is required.",
		},
		"resource": map[string]any{
			"metadata": map[string]any{
				"name": "no-team-label",
			},
		},
		"policy": map[string]any{
			"metadata": map[string]any{
				"name": "require-team-label",
			},
		},
		"rule": map[string]any{
			"name": "require-team-label",
		},
	}

	t.Run("all expressions pass", func(t *testing.T) {
		assertions := &v1alpha1.CheckAssertions{
			CEL: &v1alpha1.CheckCEL{
				Expressions: []v1alpha1.CheckExpression{
					{
						Expression: "result.status == 'fail'",
					},
					{
						Expression: "result.message.contains('team')",
					},
					{
						Expression: "resource.metadata.name == 'no-team-label'",
					},
					{
						Expression: "policy.metadata.name == 'require-team-label'",
					},
					{
						Expression: "rule.name == 'require-team-label'",
					},
				},
			},
		}

		pass, message, err := evaluateAssertions(assertions, activation, false)
		require.NoError(t, err)
		assert.True(t, pass)
		assert.Empty(t, message)
	})

	t.Run("expression failure uses message", func(t *testing.T) {
		assertions := &v1alpha1.CheckAssertions{
			CEL: &v1alpha1.CheckCEL{
				Expressions: []v1alpha1.CheckExpression{
					{
						Expression: "result.status == 'pass'",
						Message:    "Expected a failing result.",
					},
				},
			},
		}

		pass, message, err := evaluateAssertions(assertions, activation, false)
		require.NoError(t, err)
		assert.False(t, pass)
		assert.Equal(t, "Expected a failing result.", message)
	})

	t.Run("negative assertion passes when expression is false", func(t *testing.T) {
		assertions := &v1alpha1.CheckAssertions{
			CEL: &v1alpha1.CheckCEL{
				Expressions: []v1alpha1.CheckExpression{
					{
						Expression: "result.status == 'pass'",
					},
				},
			},
		}

		pass, _, err := evaluateAssertions(assertions, activation, true)
		require.NoError(t, err)
		assert.True(t, pass)
	})

	t.Run("invalid expression returns error", func(t *testing.T) {
		assertions := &v1alpha1.CheckAssertions{
			CEL: &v1alpha1.CheckCEL{
				Expressions: []v1alpha1.CheckExpression{
					{
						Expression: "result.status ==",
					},
				},
			},
		}

		_, _, err := evaluateAssertions(assertions, activation, false)
		assert.Error(t, err)
	})

	t.Run("non boolean expression returns error", func(t *testing.T) {
		assertions := &v1alpha1.CheckAssertions{
			CEL: &v1alpha1.CheckCEL{
				Expressions: []v1alpha1.CheckExpression{
					{
						Expression: "result.status",
					},
				},
			},
		}

		_, _, err := evaluateAssertions(assertions, activation, false)
		assert.Error(t, err)
	})
}

func TestCheckMatches(t *testing.T) {
	actual := map[string]any{
		"metadata": map[string]any{
			"name":      "no-team-label",
			"namespace": "default",
		},
		"spec": map[string]any{
			"enabled": true,
		},
	}

	assert.True(t, checkMatches(
		map[string]any{
			"metadata": map[string]any{
				"name": "no-team-label",
			},
		},
		actual,
	))

	assert.False(t, checkMatches(
		map[string]any{
			"metadata": map[string]any{
				"name": "different",
			},
		},
		actual,
	))
}

func TestPrintCheckResultCEL(t *testing.T) {
	color.Init(true)
	policy := &v1.ClusterPolicy{}
	policy.SetName("require-team-label")

	resource := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      "no-team-label",
				"namespace": "default",
			},
		},
	}

	rule := *engineapi.RuleFail(
		"require-team-label",
		engineapi.Validation,
		"The label `team` is required.",
		nil,
	)

	response := engineapi.NewEngineResponse(
		resource,
		engineapi.NewKyvernoPolicy(policy),
		nil,
	).WithPolicyResponse(engineapi.PolicyResponse{
		Rules: []engineapi.RuleResponse{rule},
	})

	checks := []v1alpha1.CheckResult{
		{
			Match: v1alpha1.CheckMatch{
				Resource: &v1.Any{
					Value: map[string]any{
						"name": "no-team-label",
					},
				},
				Policy: &v1.Any{
					Value: map[string]any{
						"name": "require-team-label",
					},
				},
				Rule: &v1.Any{
					Value: map[string]any{
						"name": "require-team-label",
					},
				},
			},
			Assert: &v1alpha1.CheckAssertions{
				CEL: &v1alpha1.CheckCEL{
					Expressions: []v1alpha1.CheckExpression{
						{
							Expression: "result.status == 'fail'",
						},
						{
							Expression: "result.message.contains('team')",
							Message:    "Expected violation message to mention team.",
						},
						{
							Expression: "resource.metadata.name == 'no-team-label'",
						},
						{
							Expression: "policy.metadata.name == 'require-team-label'",
						},
						{
							Expression: "rule.name == 'require-team-label'",
						},
					},
				},
			},
		},
	}

	responses := TestResponse{
		Trigger: map[string][]engineapi.EngineResponse{
			"resource.yaml": {response},
		},
	}

	var resultsTable table.Table
	rc := &resultCounts{}

	err := printCheckResult(checks, responses, rc, &resultsTable)
	require.NoError(t, err)

	require.Len(t, resultsTable.RawRows, 1)
	assert.Equal(t, 1, resultsTable.RawRows[0].ID)
	assert.Equal(t, 1, rc.Pass)
	assert.Equal(t, 0, rc.Fail)
}

func TestPrintCheckResultPreservesRowIDs(t *testing.T) {
	color.Init(true)
	policy := &v1.ClusterPolicy{}
	policy.SetName("test-policy")

	resource := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name": "test-pod",
			},
		},
	}

	rule := *engineapi.RulePass(
		"test-rule",
		engineapi.Validation,
		"ok",
		nil,
	)

	response := engineapi.NewEngineResponse(
		resource,
		engineapi.NewKyvernoPolicy(policy),
		nil,
	).WithPolicyResponse(engineapi.PolicyResponse{
		Rules: []engineapi.RuleResponse{rule},
	})

	checks := []v1alpha1.CheckResult{
		{
			Assert: &v1alpha1.CheckAssertions{
				CEL: &v1alpha1.CheckCEL{
					Expressions: []v1alpha1.CheckExpression{
						{
							Expression: "result.status == 'pass'",
						},
					},
				},
			},
		},
	}

	responses := TestResponse{
		Trigger: map[string][]engineapi.EngineResponse{
			"resource.yaml": {response},
		},
	}

	var resultsTable table.Table
	resultsTable.Add(table.Row{})
	rc := &resultCounts{}

	err := printCheckResult(checks, responses, rc, &resultsTable)
	require.NoError(t, err)

	require.Len(t, resultsTable.RawRows, 2)
	assert.Equal(t, 2, resultsTable.RawRows[1].ID)
}
