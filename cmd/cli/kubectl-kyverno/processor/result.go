package processor

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type ResultCounts struct {
	Pass  int
	Fail  int
	Warn  int
	Error int
	Skip  int
}

func (rc *ResultCounts) IncrementError(inc int) {
	rc.Error += inc
}

func (rc *ResultCounts) addGenerateResponse(response engineapi.EngineResponse) {
	genericPolicy := response.Policy()
	if gpol := genericPolicy.AsGeneratingPolicyLike(); gpol != nil {
		for _, ruleResponse := range response.PolicyResponse.Rules {
			switch ruleResponse.Status() {
			case engineapi.RuleStatusPass:
				rc.Pass++
			case engineapi.RuleStatusFail:
				rc.Fail++
			case engineapi.RuleStatusWarn:
				rc.Warn++
			case engineapi.RuleStatusSkip:
				rc.Skip++
			case engineapi.RuleStatusError:
				rc.Error++
			default:
				rc.Fail++
			}
		}
	}
}

func (rc *ResultCounts) addMutateResponse(response engineapi.EngineResponse) bool {
	printMutatedRes := false
	genericPolicy := response.Policy()

	// Handle native Kubernetes MAPs
	if mapPolicy := genericPolicy.AsMutatingAdmissionPolicy(); mapPolicy != nil {
		for _, rule := range response.PolicyResponse.Rules {
			switch rule.Status() {
			case engineapi.RuleStatusPass:
				rc.Pass++
				printMutatedRes = true
			case engineapi.RuleStatusFail:
				rc.Fail++
			case engineapi.RuleStatusSkip:
				rc.Skip++
			case engineapi.RuleStatusError:
				rc.Error++
			}
		}
	}

	// Handle MutatingPolicies
	if policy := genericPolicy.AsMutatingPolicyLike(); policy != nil {
		for _, rule := range response.PolicyResponse.Rules {
			switch rule.Status() {
			case engineapi.RuleStatusPass:
				rc.Pass++
				printMutatedRes = true
			case engineapi.RuleStatusFail:
				rc.Fail++
			case engineapi.RuleStatusSkip:
				rc.Skip++
			case engineapi.RuleStatusError:
				rc.Error++
			}
		}
	}

	return printMutatedRes
}

func (rc *ResultCounts) addValidatingAdmissionResponse(engineResponse engineapi.EngineResponse) {
	for _, ruleResp := range engineResponse.PolicyResponse.Rules {
		if ruleResp.Status() == engineapi.RuleStatusPass {
			rc.Pass++
		} else if ruleResp.Status() == engineapi.RuleStatusFail {
			rc.Fail++
		} else if ruleResp.Status() == engineapi.RuleStatusError {
			rc.Error++
		}
	}
}

func (rc *ResultCounts) AddValidatingPolicyResponse(engineResponse engineapi.EngineResponse) {
	for _, ruleResp := range engineResponse.PolicyResponse.Rules {
		if ruleResp.Status() == engineapi.RuleStatusPass {
			rc.Pass++
		} else if ruleResp.Status() == engineapi.RuleStatusFail {
			rc.Fail++
		} else if ruleResp.Status() == engineapi.RuleStatusError {
			rc.Error++
		} else if ruleResp.Status() == engineapi.RuleStatusSkip {
			rc.Skip++
		}
	}
}
