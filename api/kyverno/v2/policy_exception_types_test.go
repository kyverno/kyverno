package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExceptionContains(t *testing.T) {
	tests := []struct {
		name       string
		exception  Exception
		policyName string
		ruleName   string
		expected   bool
	}{
		{
			name:       "exact policy and rule match",
			exception:  Exception{PolicyName: "disallow-privilege-escalation", RuleNames: []string{"privilege-escalation"}},
			policyName: "disallow-privilege-escalation",
			ruleName:   "privilege-escalation",
			expected:   true,
		},
		{
			name:       "exact policy match with wildcard rule",
			exception:  Exception{PolicyName: "disallow-privilege-escalation", RuleNames: []string{"*"}},
			policyName: "disallow-privilege-escalation",
			ruleName:   "privilege-escalation",
			expected:   true,
		},
		{
			name:       "wildcard policy match with exact rule",
			exception:  Exception{PolicyName: "*", RuleNames: []string{"privilege-escalation"}},
			policyName: "disallow-privilege-escalation",
			ruleName:   "privilege-escalation",
			expected:   true,
		},
		{
			name:       "wildcard policy and wildcard rule",
			exception:  Exception{PolicyName: "*", RuleNames: []string{"*"}},
			policyName: "disallow-privilege-escalation",
			ruleName:   "privilege-escalation",
			expected:   true,
		},
		{
			name:       "policy glob pattern",
			exception:  Exception{PolicyName: "disallow-*", RuleNames: []string{"*"}},
			policyName: "disallow-privilege-escalation",
			ruleName:   "privilege-escalation",
			expected:   true,
		},
		{
			name:       "policy glob pattern no match",
			exception:  Exception{PolicyName: "disallow-*", RuleNames: []string{"*"}},
			policyName: "require-labels",
			ruleName:   "check-for-label-name",
			expected:   false,
		},
		{
			name:       "policy name mismatch",
			exception:  Exception{PolicyName: "disallow-privilege-escalation", RuleNames: []string{"privilege-escalation"}},
			policyName: "require-labels",
			ruleName:   "privilege-escalation",
			expected:   false,
		},
		{
			name:       "rule name mismatch",
			exception:  Exception{PolicyName: "disallow-privilege-escalation", RuleNames: []string{"privilege-escalation"}},
			policyName: "disallow-privilege-escalation",
			ruleName:   "other-rule",
			expected:   false,
		},
		{
			name:       "namespaced policy with wildcard",
			exception:  Exception{PolicyName: "default/*", RuleNames: []string{"*"}},
			policyName: "default/my-policy",
			ruleName:   "my-rule",
			expected:   true,
		},
		{
			name:       "question mark wildcard in policy name",
			exception:  Exception{PolicyName: "disallow-host-???", RuleNames: []string{"*"}},
			policyName: "disallow-host-pid",
			ruleName:   "any-rule",
			expected:   true,
		},
		{
			name:       "question mark wildcard no match",
			exception:  Exception{PolicyName: "disallow-host-???", RuleNames: []string{"*"}},
			policyName: "disallow-host-namespaces",
			ruleName:   "any-rule",
			expected:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.exception.Contains(tt.policyName, tt.ruleName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
