package compiler

import (
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
)

type Policy struct {
	evaluator  mutating.PolicyEvaluator
	exceptions []compiler.Exception
}
