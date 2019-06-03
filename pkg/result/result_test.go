package result

import (
	"testing"

	"gotest.tools/assert"
)

func TestAppend_TwoResultObjects(t *testing.T) {
	firstRuleApplicationResult := RuleApplicationResult{
		Reason: RequestBlocked,
		Messages: []string{
			"1. Test",
			"2. Toast",
		},
	}

	secondRuleApplicationResult := RuleApplicationResult{
		Reason: PolicyApplied,
		Messages: []string{
			"1. Kyverno",
			"2. KubePolicy",
		},
	}

	result := Append(&firstRuleApplicationResult, &secondRuleApplicationResult)
	assert.Equal(t, len(result.Children), 2)

	RuleApplicationResult, ok := result.Children[0].(*RuleApplicationResult)
	assert.Assert(t, ok)
	assert.Equal(t, RuleApplicationResult.Messages[1], "2. Toast")
}

func TestAppend_FirstObjectIsComposite(t *testing.T) {
	composite := &CompositeResult{}

	firstRuleApplicationResult := RuleApplicationResult{
		Reason: RequestBlocked,
		Messages: []string{
			"1. Test",
			"2. Toast",
		},
	}

	result := Append(composite, &firstRuleApplicationResult)

	assert.Equal(t, len(result.Children), 1)

	RuleApplicationResult, ok := result.Children[0].(*RuleApplicationResult)
	assert.Assert(t, ok)
	assert.Equal(t, RuleApplicationResult.Messages[1], "2. Toast")
}
