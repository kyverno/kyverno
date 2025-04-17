package v2

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/autogen/v1"
)

type v2 struct{}

func New() v2 {
	return v2{}
}

func (a v2) GetAutogenRuleNames(p kyvernov1.PolicyInterface) []string {
	return GetAutogenRuleNames(p)
}

func (a v2) GetAutogenKinds(p kyvernov1.PolicyInterface) []string {
	return GetAutogenKinds(p)
}

func (a v2) ComputeRules(p kyvernov1.PolicyInterface, kind string) []kyvernov1.Rule {
	return v1.ComputeRules(p, kind)
}
