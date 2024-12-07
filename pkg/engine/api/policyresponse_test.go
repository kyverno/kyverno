package api

import (
	"testing"
)

func TestPolicyResponse_Add(t *testing.T) {
	tests := []struct {
		name               string
		initialStats       PolicyStats
		ruleResponses      []RuleResponse
		expectedStats      PolicyStats
		expectedPassCount  int
		expectedFailCount  int
		expectedErrorCount int
	}{
		{
			name: "Add pass and fail rules",
			initialStats: PolicyStats{
				rulesAppliedCount: 0,
				rulesErrorCount:   0,
			},
			ruleResponses: []RuleResponse{
				{status: RuleStatusPass},
				{status: RuleStatusFail},
			},
			expectedStats: PolicyStats{
				rulesAppliedCount: 2,
				rulesErrorCount:   0,
			},
			expectedPassCount:  2,
			expectedFailCount:  0,
			expectedErrorCount: 0,
		},
		{
			name: "Add rule with error",
			initialStats: PolicyStats{
				rulesAppliedCount: 0,
				rulesErrorCount:   0,
			},
			ruleResponses: []RuleResponse{
				{status: RuleStatusError},
			},
			expectedStats: PolicyStats{
				rulesAppliedCount: 0,
				rulesErrorCount:   1,
			},
			expectedPassCount:  0,
			expectedFailCount:  0,
			expectedErrorCount: 1,
		},
		{
			name: "Add multiple rules with different statuses",
			initialStats: PolicyStats{
				rulesAppliedCount: 0,
				rulesErrorCount:   0,
			},
			ruleResponses: []RuleResponse{
				{status: RuleStatusPass},
				{status: RuleStatusFail},
				{status: RuleStatusError},
			},
			expectedStats: PolicyStats{
				rulesAppliedCount: 2,
				rulesErrorCount:   1,
			},
			expectedPassCount:  2,
			expectedFailCount:  0,
			expectedErrorCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PolicyResponse{stats: tt.initialStats}

			pr.Add(ExecutionStats{}, tt.ruleResponses...)

			if pr.stats.rulesAppliedCount != tt.expectedStats.rulesAppliedCount {
				t.Errorf("expected applied count %d, got %d", tt.expectedStats.rulesAppliedCount, pr.stats.rulesAppliedCount)
			}

			if pr.stats.rulesErrorCount != tt.expectedStats.rulesErrorCount {
				t.Errorf("expected error count %d, got %d", tt.expectedStats.rulesErrorCount, pr.stats.rulesErrorCount)
			}
		})
	}
}

func TestPolicyResponse_RulesAppliedCount(t *testing.T) {
	tests := []struct {
		name          string
		initialStats  PolicyStats
		expectedCount int
	}{
		{
			name: "Check applied rules count",
			initialStats: PolicyStats{
				rulesAppliedCount: 5,
				rulesErrorCount:   2,
			},
			expectedCount: 5,
		},
		{
			name: "Check applied rules count after adding rules",
			initialStats: PolicyStats{
				rulesAppliedCount: 2,
				rulesErrorCount:   1,
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PolicyResponse{stats: tt.initialStats}

			got := pr.RulesAppliedCount()

			if got != tt.expectedCount {
				t.Errorf("expected %d, got %d", tt.expectedCount, got)
			}
		})
	}
}

func TestPolicyResponse_RulesErrorCount(t *testing.T) {
	tests := []struct {
		name               string
		initialStats       PolicyStats
		expectedErrorCount int
	}{
		{
			name: "Check error rules count",
			initialStats: PolicyStats{
				rulesAppliedCount: 5,
				rulesErrorCount:   3,
			},
			expectedErrorCount: 3,
		},
		{
			name: "Check error rules count after adding rules",
			initialStats: PolicyStats{
				rulesAppliedCount: 4,
				rulesErrorCount:   2,
			},
			expectedErrorCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PolicyResponse{stats: tt.initialStats}

			got := pr.RulesErrorCount()

			if got != tt.expectedErrorCount {
				t.Errorf("expected %d, got %d", tt.expectedErrorCount, got)
			}
		})
	}
}
