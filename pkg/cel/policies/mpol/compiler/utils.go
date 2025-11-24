package compiler

import (
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	cel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
)

func ConvertVariables(in []admissionregistrationv1beta1.Variable) []cel.NamedExpressionAccessor {
	namedExpressions := make([]cel.NamedExpressionAccessor, len(in))
	for i, variable := range in {
		namedExpressions[i] = &validating.Variable{
			Name:       variable.Name,
			Expression: variable.Expression,
		}
	}
	return namedExpressions
}
