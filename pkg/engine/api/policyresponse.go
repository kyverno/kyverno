package api

import "time"

// PolicyResponse policy application response
type PolicyResponse struct {
	// Stats contains policy statistics
	Stats PolicyStats
	// Rules contains policy rules responses
	Rules []RuleResponse
}

func (pr *PolicyResponse) Add(startTime, endTime time.Time, response RuleResponse) {
	pr.Rules = append(pr.Rules, response.WithStats(startTime, endTime))
	if response.Status == RuleStatusPass || response.Status == RuleStatusFail {
		pr.Stats.RulesAppliedCount++
	} else if response.Status == RuleStatusError {
		pr.Stats.RulesErrorCount++
	}
}

func NewPolicyResponse() PolicyResponse {
	return PolicyResponse{}
}
