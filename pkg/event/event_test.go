package event

import (
	"testing"

	"gotest.tools/assert"
)

func TestAppend_TwoKyvernoEventObjects(t *testing.T) {
	firstRuleEvent := RuleEvent{
		Reason: RequestBlocked,
		Messages: []string{
			"1. Test",
			"2. Toast",
		},
	}

	secondRuleEvent := RuleEvent{
		Reason: PolicyApplied,
		Messages: []string{
			"1. Kyverno",
			"2. KubePolicy",
		},
	}

	result := Append(&firstRuleEvent, &secondRuleEvent)
	assert.Equal(t, len(result.Children), 2)

	ruleEvent, ok := result.Children[0].(*RuleEvent)
	assert.Assert(t, ok)
	assert.Equal(t, ruleEvent.Messages[1], "2. Toast")
}

func TestAppend_FirstObjectIsComposite(t *testing.T) {
	composite := &CompositeEvent{}

	firstRuleEvent := RuleEvent{
		Reason: RequestBlocked,
		Messages: []string{
			"1. Test",
			"2. Toast",
		},
	}

	result := Append(composite, &firstRuleEvent)

	assert.Equal(t, len(result.Children), 1)

	ruleEvent, ok := result.Children[0].(*RuleEvent)
	assert.Assert(t, ok)
	assert.Equal(t, ruleEvent.Messages[1], "2. Toast")
}
