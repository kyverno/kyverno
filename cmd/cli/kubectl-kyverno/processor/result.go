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
	if kyvernoPolicy := genericPolicy.AsKyvernoPolicy(); kyvernoPolicy != nil {
		for _, policyRule := range autogen.Default.ComputeRules(kyvernoPolicy, "") {
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
	} else if gpol := genericPolicy.AsGeneratingPolicy(); gpol != nil {
		for _, ruleResponse := range response.PolicyResponse.Rules {
			if ruleResponse.Status() == engineapi.RuleStatusPass {
				rc.Pass++
			} else if ruleResponse.Status() == engineapi.RuleStatusFail {
				rc.Fail++
			}
		}
	}
}

func (rc *ResultCounts) addMutateResponse(response engineapi.EngineResponse) bool {
	printMutatedRes := false
	genericPolicy := response.Policy()

	// Handle Kyverno mutate policies
	if kyvernoPolicy := genericPolicy.AsKyvernoPolicy(); kyvernoPolicy != nil {
		// Check if it has at least one mutate rule
		hasMutate := false
		for _, rule := range autogen.Default.ComputeRules(kyvernoPolicy, "") {
			if rule.HasMutate() {
				hasMutate = true
				break
			}
		}
		if !hasMutate {
			return false
		}

		for _, policyRule := range autogen.Default.ComputeRules(kyvernoPolicy, "") {
			for _, responseRule := range response.PolicyResponse.Rules {
				if policyRule.Name == responseRule.Name() {
					switch responseRule.Status() {
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
		}
		return printMutatedRes
	}

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
	if policy := genericPolicy.AsMutatingPolicy(); policy != nil {
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
