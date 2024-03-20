package processor

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

type policyExceptionSelector struct {
	exceptions []*kyvernov2beta1.PolicyException
}

func (l *policyExceptionSelector) GetPolicyExceptionsByPolicyRulePair(policyName, ruleName string) ([]*kyvernov2beta1.PolicyException, error) {
	var out []*kyvernov2beta1.PolicyException
	matches := false
	for _, polex := range l.exceptions {
		matches = false
		for _, exception := range polex.Spec.Exceptions {
			if matches {
				break
			}
			for _, rule := range exception.RuleNames {
				if exception.PolicyName == policyName && rule == ruleName {
					out = append(out, polex)
					matches = true
					break
				}
			}
		}
	}
	return out, nil
}
