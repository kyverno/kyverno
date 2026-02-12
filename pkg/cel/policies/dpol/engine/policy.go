package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
)

type Policy struct {
	Policy         policiesv1beta1.DeletingPolicyLike
	CompiledPolicy *compiler.Policy
}
