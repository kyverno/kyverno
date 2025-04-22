package policy

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type Compiler interface {
	CompileValidating(*policiesv1alpha1.ValidatingPolicy, []*policiesv1alpha1.PolicyException) (CompiledPolicy, field.ErrorList)
}
