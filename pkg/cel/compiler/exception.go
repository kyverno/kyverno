package compiler

import (
	"github.com/google/cel-go/cel"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
)

type Exception struct {
	Exception       *policiesv1alpha1.PolicyException
	MatchConditions []cel.Program
}
