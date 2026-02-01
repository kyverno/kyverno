package handlers

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
)

// TestWithError verifies the error response helper function creates proper error responses
func TestWithError(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "test-rule",
	}
	err := errors.New("test error")

	responses := WithError(rule, engineapi.Validation, "test message", err)

	assert.Len(t, responses, 1)
	assert.Equal(t, "test-rule", responses[0].Name())
	assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
	assert.Equal(t, engineapi.Validation, responses[0].RuleType())
	assert.Contains(t, responses[0].Message(), "test message")
	assert.Contains(t, responses[0].Message(), "test error")
}

func TestWithError_NilError(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "test-rule",
	}

	responses := WithError(rule, engineapi.Validation, "test message", nil)

	assert.Len(t, responses, 1)
	assert.Equal(t, "test-rule", responses[0].Name())
	assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
	assert.Equal(t, engineapi.Validation, responses[0].RuleType())
	assert.Contains(t, responses[0].Message(), "test message")
}

func TestWithSkip(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "test-rule",
	}

	responses := WithSkip(rule, engineapi.Validation, "test skip message")

	assert.Len(t, responses, 1)
	assert.Equal(t, "test-rule", responses[0].Name())
	assert.Equal(t, engineapi.RuleStatusSkip, responses[0].Status())
	assert.Equal(t, engineapi.Validation, responses[0].RuleType())
	assert.Equal(t, "test skip message", responses[0].Message())
}

func TestWithPass(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "test-rule",
	}

	responses := WithPass(rule, engineapi.Validation, "test pass message")

	assert.Len(t, responses, 1)
	assert.Equal(t, "test-rule", responses[0].Name())
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
	assert.Equal(t, engineapi.Validation, responses[0].RuleType())
	assert.Equal(t, "test pass message", responses[0].Message())
}

func TestWithFail(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "test-rule",
	}

	responses := WithFail(rule, engineapi.Validation, "test fail message")

	assert.Len(t, responses, 1)
	assert.Equal(t, "test-rule", responses[0].Name())
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Equal(t, engineapi.Validation, responses[0].RuleType())
	assert.Equal(t, "test fail message", responses[0].Message())
}

func TestWithResponses_NilInput(t *testing.T) {
	responses := WithResponses(nil, nil, nil)

	assert.Nil(t, responses)
}

func TestWithResponses_EmptyInput(t *testing.T) {
	responses := WithResponses()

	assert.Nil(t, responses)
}

func TestWithResponses_MixedInput(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "test-rule",
	}

	ruleResponse := engineapi.RulePass(rule.Name, engineapi.Validation, "test", rule.ReportProperties)

	responses := WithResponses(nil, ruleResponse, nil)

	assert.Len(t, responses, 1)
}

func TestWithResponses_MultipleResponses(t *testing.T) {
	rule1 := kyvernov1.Rule{Name: "rule-1"}
	rule2 := kyvernov1.Rule{Name: "rule-2"}

	resp1 := engineapi.RulePass(rule1.Name, engineapi.Validation, "pass", rule1.ReportProperties)
	resp2 := engineapi.RuleFail(rule2.Name, engineapi.Validation, "fail", rule2.ReportProperties)

	responses := WithResponses(resp1, resp2)

	assert.Len(t, responses, 2)
	assert.Equal(t, "rule-1", responses[0].Name())
	assert.Equal(t, "rule-2", responses[1].Name())
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
	assert.Equal(t, engineapi.RuleStatusFail, responses[1].Status())
}

func TestWithError_DifferentRuleTypes(t *testing.T) {
	ruleTypes := []engineapi.RuleType{
		engineapi.Validation,
		engineapi.Mutation,
		engineapi.Generation,
		engineapi.ImageVerify,
	}

	for _, ruleType := range ruleTypes {
		t.Run(string(ruleType), func(t *testing.T) {
			rule := kyvernov1.Rule{Name: "test-rule"}
			err := errors.New("test error")

			responses := WithError(rule, ruleType, "error message", err)

			assert.Len(t, responses, 1)
			assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
			assert.Equal(t, ruleType, responses[0].RuleType())
		})
	}
}

func TestWithSkip_DifferentRuleTypes(t *testing.T) {
	ruleTypes := []engineapi.RuleType{
		engineapi.Validation,
		engineapi.Mutation,
		engineapi.Generation,
		engineapi.ImageVerify,
	}

	for _, ruleType := range ruleTypes {
		t.Run(string(ruleType), func(t *testing.T) {
			rule := kyvernov1.Rule{Name: "test-rule"}

			responses := WithSkip(rule, ruleType, "skip message")

			assert.Len(t, responses, 1)
			assert.Equal(t, engineapi.RuleStatusSkip, responses[0].Status())
			assert.Equal(t, ruleType, responses[0].RuleType())
		})
	}
}

func TestWithPass_DifferentRuleTypes(t *testing.T) {
	ruleTypes := []engineapi.RuleType{
		engineapi.Validation,
		engineapi.Mutation,
		engineapi.Generation,
		engineapi.ImageVerify,
	}

	for _, ruleType := range ruleTypes {
		t.Run(string(ruleType), func(t *testing.T) {
			rule := kyvernov1.Rule{Name: "test-rule"}

			responses := WithPass(rule, ruleType, "pass message")

			assert.Len(t, responses, 1)
			assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
			assert.Equal(t, ruleType, responses[0].RuleType())
		})
	}
}

func TestWithFail_DifferentRuleTypes(t *testing.T) {
	ruleTypes := []engineapi.RuleType{
		engineapi.Validation,
		engineapi.Mutation,
		engineapi.Generation,
		engineapi.ImageVerify,
	}

	for _, ruleType := range ruleTypes {
		t.Run(string(ruleType), func(t *testing.T) {
			rule := kyvernov1.Rule{Name: "test-rule"}

			responses := WithFail(rule, ruleType, "fail message")

			assert.Len(t, responses, 1)
			assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
			assert.Equal(t, ruleType, responses[0].RuleType())
		})
	}
}

func TestWithError_EmptyRuleName(t *testing.T) {
	rule := kyvernov1.Rule{Name: ""}
	err := errors.New("error")

	responses := WithError(rule, engineapi.Validation, "message", err)

	assert.Len(t, responses, 1)
	assert.Empty(t, responses[0].Name())
}

func TestWithError_WithReportProperties(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "test-rule",
		ReportProperties: map[string]string{
			"key": "value",
		},
	}
	err := errors.New("error")

	responses := WithError(rule, engineapi.Validation, "message", err)

	assert.Len(t, responses, 1)
	assert.Equal(t, "test-rule", responses[0].Name())
	props := responses[0].Properties()
	assert.NotNil(t, props)
	assert.Equal(t, "value", props["key"])
}
