package engine

import (
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
)

type Policy struct {
	Policy         policiesv1beta1.MutatingPolicyLike
	CompiledPolicy *compiler.Policy
}
