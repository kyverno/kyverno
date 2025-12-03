package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
)

type Policy struct {
	Policy         policiesv1beta1.GeneratingPolicyLike
	Exceptions     []*policiesv1alpha1.PolicyException
	CompiledPolicy *compiler.Policy
}
