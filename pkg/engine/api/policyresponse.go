package api

// PolicyResponse policy application response
type PolicyResponse struct {
	// Stats contains policy statistics
	Stats PolicyStats
	// Rules contains policy rules responses
	Rules []RuleResponse
}

func (pr *PolicyResponse) Add(rr RuleResponse) {
	pr.Rules = append(pr.Rules, rr)
	if rr.Status == RuleStatusPass || rr.Status == RuleStatusFail {
		pr.Stats.RulesAppliedCount++
	} else if rr.Status == RuleStatusError {
		pr.Stats.RulesErrorCount++
	}
}

func NewPolicyResponse() PolicyResponse {
	return PolicyResponse{}
}
