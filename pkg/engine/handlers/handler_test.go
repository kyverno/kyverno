package handlers

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
)

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
