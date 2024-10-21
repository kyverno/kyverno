package autogen

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	autogenv1 "github.com/kyverno/kyverno/pkg/autogen/v1"
	autogenv2 "github.com/kyverno/kyverno/pkg/autogen/v2"
)

type Autogen interface {
	GetAutogenRuleNames(kyvernov1.PolicyInterface) []string
	GetAutogenKinds(kyvernov1.PolicyInterface) []string
	ComputeRules(kyvernov1.PolicyInterface, string) []kyvernov1.Rule
}

var (
	V1      Autogen = autogenv1.New()
	V2      Autogen = autogenv2.New()
	Default Autogen = V1
)
