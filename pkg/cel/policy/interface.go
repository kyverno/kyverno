package policy

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type Compiler interface {
	CompileValidating(policy *policiesv1alpha1.ValidatingPolicy, exceptions []policiesv1alpha1.CELPolicyException) (CompiledPolicy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compiler{}
}

type compiler struct{}
