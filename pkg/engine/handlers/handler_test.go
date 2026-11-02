package handlers

import (
	"testing"

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
	ruleName := "test-rule"

	ruleResponse := engineapi.RulePass(ruleName, engineapi.Validation, "test", nil)

	responses := WithResponses(nil, ruleResponse, nil)

	assert.Len(t, responses, 1)
}

func TestWithResponses_MultipleResponses(t *testing.T) {
	resp1 := engineapi.RulePass("rule-1", engineapi.Validation, "pass", nil)
	resp2 := engineapi.RuleFail("rule-2", engineapi.Validation, "fail", nil)

	responses := WithResponses(resp1, resp2)

	assert.Len(t, responses, 2)
	assert.Equal(t, "rule-1", responses[0].Name())
	assert.Equal(t, "rule-2", responses[1].Name())
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
	assert.Equal(t, engineapi.RuleStatusFail, responses[1].Status())
}
