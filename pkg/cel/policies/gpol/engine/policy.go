package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
)

type Policy struct {
	Policy         policiesv1beta1.GeneratingPolicyLike
	Exceptions     []*policiesv1beta1.PolicyException
	CompiledPolicy *compiler.Policy
}
