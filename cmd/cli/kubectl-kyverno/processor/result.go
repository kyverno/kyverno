package processor

import (
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy/annotations"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/api/admissionregistration/v1alpha1"
)

type ResultCounts struct {
	pass int
	fail int
	warn int
	err  int
	skip int
}

func (rc ResultCounts) Pass() int  { return rc.pass }
func (rc ResultCounts) Fail() int  { return rc.fail }
func (rc ResultCounts) Warn() int  { return rc.warn }
func (rc ResultCounts) Error() int { return rc.err }
func (rc ResultCounts) Skip() int  { return rc.skip }

func (rc *ResultCounts) IncrementError(inc int) {
	rc.err += inc
}

func (rc *ResultCounts) addEngineResponses(auditWarn bool, policyReport bool, resourcePath string, responses ...engineapi.EngineResponse) {
	for _, response := range responses {
		rc.addEngineResponse(auditWarn, policyReport, resourcePath, response)
	}
}

func (rc *ResultCounts) addEngineResponse(auditWarn bool, policyReport bool, resourcePath string, response engineapi.EngineResponse) {
	printCount := 0
	if !response.IsEmpty() {
		genericPolicy := response.Policy()
		if polType := genericPolicy.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
			return
		}
		policy := genericPolicy.AsKyvernoPolicy()
		scored := annotations.Scored(policy.GetAnnotations())
		for i, rule := range autogen.ComputeRules(policy) {
			if rule.HasValidate() || rule.HasVerifyImageChecks() || rule.HasVerifyImages() {
				for _, valResponseRule := range response.PolicyResponse.Rules {
					if rule.Name == valResponseRule.Name() {
						switch valResponseRule.Status() {
						case engineapi.RuleStatusPass:
							rc.pass++
						case engineapi.RuleStatusFail:
							auditWarning := false
							if !scored {
								rc.warn++
								break
							} else if auditWarn && response.GetValidationFailureAction().Audit() {
								auditWarning = true
								rc.warn++
							} else {
								rc.fail++
							}
							if !policyReport {
								if printCount < 1 {
									if auditWarning {
										fmt.Printf("\npolicy %s -> resource %s failed as audit warning: \n", policy.GetName(), resourcePath)
									} else {
										fmt.Printf("\npolicy %s -> resource %s failed: \n", policy.GetName(), resourcePath)
									}
									printCount++
								}
								fmt.Printf("%d. %s: %s \n", i+1, valResponseRule.Name(), valResponseRule.Message())
							}
						case engineapi.RuleStatusError:
							rc.err++
						case engineapi.RuleStatusWarn:
							rc.warn++
						case engineapi.RuleStatusSkip:
							rc.skip++
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
	for _, policyRule := range autogen.ComputeRules(policy) {
		for _, ruleResponse := range response.PolicyResponse.Rules {
			if policyRule.Name == ruleResponse.Name() {
				if ruleResponse.Status() == engineapi.RuleStatusPass {
					rc.pass++
				} else {
					if auditWarn && response.GetValidationFailureAction().Audit() {
						rc.warn++
					} else {
						rc.fail++
					}
				}
				continue
			}
		}
	}
}

func (rc *ResultCounts) addMutateResponse(resourcePath string, response engineapi.EngineResponse) bool {
	genericPolicy := response.Policy()
	if polType := genericPolicy.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
		return false
	}
	policy := genericPolicy.AsKyvernoPolicy()
	var policyHasMutate bool
	for _, rule := range autogen.ComputeRules(policy) {
		if rule.HasMutate() {
			policyHasMutate = true
		}
	}
	if !policyHasMutate {
		return false
	}
	printCount := 0
	printMutatedRes := false
	for i, policyRule := range autogen.ComputeRules(policy) {
		for _, mutateResponseRule := range response.PolicyResponse.Rules {
			if policyRule.Name == mutateResponseRule.Name() {
				if mutateResponseRule.Status() == engineapi.RuleStatusPass {
					rc.pass++
					printMutatedRes = true
				} else if mutateResponseRule.Status() == engineapi.RuleStatusSkip {
					fmt.Printf("\nskipped mutate policy %s -> resource %s", policy.GetName(), resourcePath)
					rc.skip++
				} else if mutateResponseRule.Status() == engineapi.RuleStatusError {
					fmt.Printf("\nerror while applying mutate policy %s -> resource %s\nerror: %s", policy.GetName(), resourcePath, mutateResponseRule.Message())
					rc.err++
				} else {
					if printCount < 1 {
						fmt.Printf("\nfailed to apply mutate policy %s -> resource %s", policy.GetName(), resourcePath)
						printCount++
					}
					fmt.Printf("%d. %s - %s \n", i+1, mutateResponseRule.Name(), mutateResponseRule.Message())
					rc.fail++
				}
				continue
			}
		}
	}
	return printMutatedRes
}

func (rc *ResultCounts) addValidatingAdmissionResponse(vap v1alpha1.ValidatingAdmissionPolicy, engineResponse engineapi.EngineResponse) {
	for _, ruleResp := range engineResponse.PolicyResponse.Rules {
		if ruleResp.Status() == engineapi.RuleStatusPass {
			rc.pass++
		} else if ruleResp.Status() == engineapi.RuleStatusFail {
			rc.fail++
		} else if ruleResp.Status() == engineapi.RuleStatusError {
			rc.err++
		}
	}
}
