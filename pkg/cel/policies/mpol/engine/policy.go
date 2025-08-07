package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
)

type Policy struct {
	Policy         policiesv1alpha1.MutatingPolicy
	CompiledPolicy *compiler.Policy
}
