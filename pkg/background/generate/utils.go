package generate

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func newResourceSpec(genAPIVersion, genKind, genNamespace, genName string) kyvernov1.ResourceSpec {
	return kyvernov1.ResourceSpec{
		APIVersion: genAPIVersion,
		Kind:       genKind,
		Namespace:  genNamespace,
		Name:       genName,
	}
}

func buildPolicyWithAppliedRules(policy kyvernov1.PolicyInterface, expect string) (kyvernov1.PolicyInterface, bool) {
	var rule *kyvernov1.Rule
	p := policy.CreateDeepCopy()
	for j := range p.GetSpec().Rules {
		if p.GetSpec().Rules[j].Name == expect {
			rule = &p.GetSpec().Rules[j]
			break
		}
	}
	if rule == nil {
		return nil, false
	}

	p.GetSpec().SetRules([]kyvernov1.Rule{*rule})
	return p, true
}
