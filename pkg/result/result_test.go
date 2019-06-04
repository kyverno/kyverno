package result

import (
	"testing"

	"gotest.tools/assert"
)

func TestAppend_TwoResultObjects(t *testing.T) {
	firstRuleApplicationResult := RuleApplicationResult{
		Reason: Failed,
		Messages: []string{
			"1. Test",
			"2. Toast",
		},
	}

	secondRuleApplicationResult := RuleApplicationResult{
		Reason: Success,
		Messages: []string{
			"1. Kyverno",
			"2. KubePolicy",
		},
	}

	result := Append(&firstRuleApplicationResult, &secondRuleApplicationResult)
	composite, ok := result.(*CompositeResult)
	assert.Assert(t, ok)
	assert.Equal(t, len(composite.Children), 2)

	RuleApplicationResult, ok := composite.Children[0].(*RuleApplicationResult)
	assert.Assert(t, ok)
	assert.Equal(t, RuleApplicationResult.Messages[1], "2. Toast")
}

func TestAppend_FirstObjectIsComposite(t *testing.T) {
	composite := &CompositeResult{}

	firstRuleApplicationResult := RuleApplicationResult{
		Reason: Failed,
		Messages: []string{
			"1. Test",
			"2. Toast",
		},
	}

	result := Append(composite, &firstRuleApplicationResult)
	composite, ok := result.(*CompositeResult)
	assert.Equal(t, len(composite.Children), 1)

	RuleApplicationResult, ok := composite.Children[0].(*RuleApplicationResult)
	assert.Assert(t, ok)
	assert.Equal(t, RuleApplicationResult.Messages[1], "2. Toast")
}
