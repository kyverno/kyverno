package processor

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy/annotations"
	"github.com/kyverno/kyverno/pkg/autogen"
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

func (rc *ResultCounts) addEngineResponses(auditWarn bool, responses ...engineapi.EngineResponse) {
	for _, response := range responses {
		rc.addEngineResponse(auditWarn, response)
	}
}

func (rc *ResultCounts) addEngineResponse(auditWarn bool, response engineapi.EngineResponse) {
	if !response.IsEmpty() {
		genericPolicy := response.Policy()
		if genericPolicy.AsKyvernoPolicy() == nil {
			return
		}
		policy := genericPolicy.AsKyvernoPolicy()
		scored := annotations.Scored(policy.GetAnnotations())
		for _, rule := range autogen.Default.ComputeRules(policy, "") {
			if rule.HasValidate() || rule.HasVerifyImageChecks() || rule.HasVerifyImages() {
				for _, valResponseRule := range response.PolicyResponse.Rules {
					if rule.Name == valResponseRule.Name() {
						switch valResponseRule.Status() {
						case engineapi.RuleStatusPass:
							rc.Pass++
						case engineapi.RuleStatusFail:
							if !scored {
								rc.Warn++
								break
							} else if auditWarn && response.GetValidationFailureAction().Audit() {
								rc.Warn++
							} else {
								rc.Fail++
							}
						case engineapi.RuleStatusError:
							rc.Error++
						case engineapi.RuleStatusWarn:
							rc.Warn++
						case engineapi.RuleStatusSkip:
							rc.Skip++
						}
						continue
					}
				}
			}
		}
	}
}

func (rc *ResultCounts) addGenerateResponse(response engineapi.EngineResponse) {
	genericPolicy := response.Policy()
	if genericPolicy.AsKyvernoPolicy() == nil {
		return
	}
	policy := genericPolicy.AsKyvernoPolicy()
	for _, policyRule := range autogen.Default.ComputeRules(policy, "") {
		for _, ruleResponse := range response.PolicyResponse.Rules {
			if policyRule.Name == ruleResponse.Name() {
				if ruleResponse.Status() == engineapi.RuleStatusPass {
					rc.Pass++
				} else {
					rc.Fail++
				}
				continue
			}
		}
	}
}

func (rc *ResultCounts) addMutateResponse(response engineapi.EngineResponse) bool {
	printed := false
	// for each rule in the response, if it's a mutation, tally it
	for _, rule := range response.PolicyResponse.Rules {
		if rule.RuleType() != engineapi.Mutation {
			continue
		}
		switch rule.Status() {
		case engineapi.RuleStatusPass:
			rc.Pass++
			printed = true
		case engineapi.RuleStatusFail:
			rc.Fail++
		case engineapi.RuleStatusError:
			rc.Error++
		case engineapi.RuleStatusSkip:
			rc.Skip++
		}
	}
	return printed
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
