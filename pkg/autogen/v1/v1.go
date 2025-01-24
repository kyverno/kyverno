package v1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

type v1 struct{}

func New() v1 {
	return v1{}
}

func (a v1) GetAutogenRuleNames(p kyvernov1.PolicyInterface) []string {
	var out []string //nolint:prealloc
	for _, rule := range a.ComputeRules(p, "") {
		out = append(out, rule.Name)
	}
	return out
}

func (a v1) GetAutogenKinds(p kyvernov1.PolicyInterface) []string {
	var out []string
	for _, rule := range a.ComputeRules(p, "") {
		out = append(out, rule.MatchResources.GetKinds()...)
	}
	return out
}

func (a v1) ComputeRules(p kyvernov1.PolicyInterface, kind string) []kyvernov1.Rule {
	return ComputeRules(p, kind)
}
