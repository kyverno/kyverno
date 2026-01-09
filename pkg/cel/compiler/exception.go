package compiler

import (
	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

type Exception struct {
	Exception       *policiesv1beta1.PolicyException
	MatchConditions []cel.Program
}
