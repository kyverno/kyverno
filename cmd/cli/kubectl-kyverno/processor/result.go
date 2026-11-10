package processor

import (
	"slices"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

// addValidatingAdmissionResponse records rule results for a validating admission policy response.
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

// IsAuditOnlyValidatingPolicy checks whether the policy is a ValidatingPolicy or ImageValidatingPolicy
// with a non-empty validationActions list that does not contain Deny.
func IsAuditOnlyValidatingPolicy(pol engineapi.GenericPolicy) bool {
	if pol == nil {
		return false
	}
	if vpol := pol.AsValidatingPolicyLike(); vpol != nil && vpol.GetSpec() != nil {
		actions := vpol.GetSpec().ValidationActions()
		return len(actions) > 0 && !slices.Contains(actions, admissionregistrationv1.Deny)
	}
	if ivpol := pol.AsImageValidatingPolicyLike(); ivpol != nil && ivpol.GetSpec() != nil {
		actions := ivpol.GetSpec().ValidationActions()
		return len(actions) > 0 && !slices.Contains(actions, admissionregistrationv1.Deny)
	}
	return false
}

// AddValidatingPolicyResponse records rule results for a validating policy response.
// When auditWarn is true and the policy is audit-only (specifies a non-empty list of validation actions
// without the Deny action), rule failures are counted under Warn instead of Fail.
func (rc *ResultCounts) AddValidatingPolicyResponse(auditWarn bool, engineResponse engineapi.EngineResponse) {
	audit := IsAuditOnlyValidatingPolicy(engineResponse.Policy())
	for _, ruleResp := range engineResponse.PolicyResponse.Rules {
		switch ruleResp.Status() {
		case engineapi.RuleStatusPass:
			rc.Pass++
		case engineapi.RuleStatusFail:
			if auditWarn && audit {
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
	}
}
