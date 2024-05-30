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
		if polType := genericPolicy.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
			return
		}
		policy := genericPolicy.AsKyvernoPolicy()
		scored := annotations.Scored(policy.GetAnnotations())
		for _, rule := range autogen.ComputeRules(policy, "") {
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

func (rc *ResultCounts) addGenerateResponse(auditWarn bool, response engineapi.EngineResponse) {
	genericPolicy := response.Policy()
	if polType := genericPolicy.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
		return
	}
	policy := genericPolicy.AsKyvernoPolicy()
	for _, policyRule := range autogen.ComputeRules(policy, "") {
		for _, ruleResponse := range response.PolicyResponse.Rules {
			if policyRule.Name == ruleResponse.Name() {
				if ruleResponse.Status() == engineapi.RuleStatusPass {
					rc.Pass++
				} else {
					if auditWarn && response.GetValidationFailureAction().Audit() {
						rc.Warn++
					} else {
						rc.Fail++
					}
				}
				continue
			}
		}
	}
}

func (rc *ResultCounts) addMutateResponse(response engineapi.EngineResponse) bool {
	genericPolicy := response.Policy()
	if polType := genericPolicy.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
		return false
	}
	policy := genericPolicy.AsKyvernoPolicy()
	var policyHasMutate bool
	for _, rule := range autogen.ComputeRules(policy, "") {
		if rule.HasMutate() {
			policyHasMutate = true
		}
	}
	if !policyHasMutate {
		return false
	}
	printMutatedRes := false
	for _, policyRule := range autogen.ComputeRules(policy, "") {
		for _, mutateResponseRule := range response.PolicyResponse.Rules {
			if policyRule.Name == mutateResponseRule.Name() {
				if mutateResponseRule.Status() == engineapi.RuleStatusPass {
					rc.Pass++
					printMutatedRes = true
				} else if mutateResponseRule.Status() == engineapi.RuleStatusSkip {
					rc.Skip++
				} else if mutateResponseRule.Status() == engineapi.RuleStatusError {
					rc.Error++
				} else {
					rc.Fail++
				}
				continue
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
