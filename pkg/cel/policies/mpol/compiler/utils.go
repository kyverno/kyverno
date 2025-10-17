package compiler

import (
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	cel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
)

func ConvertVariables(in []admissionregistrationv1alpha1.Variable) []cel.NamedExpressionAccessor {
	namedExpressions := make([]cel.NamedExpressionAccessor, len(in))
	for i, variable := range in {
		namedExpressions[i] = &validating.Variable{
			Name:       variable.Name,
			Expression: variable.Expression,
		}
	}
	return namedExpressions
}
