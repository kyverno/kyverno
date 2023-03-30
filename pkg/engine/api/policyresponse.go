package api

// PolicyResponse policy application response
type PolicyResponse struct {
	// Stats contains policy statistics
	Stats PolicyStats
	// Rules contains policy rules responses
	Rules []RuleResponse
}

func (pr *PolicyResponse) Add(response RuleResponse) {
	// if the response has not been marked done yet
	if !response.IsDone() {
		response = response.DoneNow()
	}
	pr.Rules = append(pr.Rules, response)
	if response.Status == RuleStatusPass || response.Status == RuleStatusFail {
		pr.Stats.RulesAppliedCount++
	} else if response.Status == RuleStatusError {
		pr.Stats.RulesErrorCount++
	}
}

func NewPolicyResponse() PolicyResponse {
	return PolicyResponse{}
}
