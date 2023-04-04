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
	status := response.ZStatus()
	if status == RuleStatusPass || status == RuleStatusFail {
		pr.Stats.RulesAppliedCount++
	} else if status == RuleStatusError {
		pr.Stats.RulesErrorCount++
	}
}

func NewPolicyResponse() PolicyResponse {
	return PolicyResponse{}
}
