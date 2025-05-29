package compiler

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	cel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
)

func convertVariables(in []admissionregistrationv1alpha1.Variable) []cel.NamedExpressionAccessor {
	namedExpressions := make([]cel.NamedExpressionAccessor, len(in))
	for i, variable := range in {
		namedExpressions[i] = &mutating.Variable{
			Name:       variable.Name,
			Expression: variable.Expression,
		}
	}
	return namedExpressions
}

func toV1FailurePolicy(failurePolicy *admissionregistrationv1alpha1.FailurePolicyType) *admissionregistrationv1.FailurePolicyType {
	if failurePolicy == nil {
		return nil
	}
	fp := admissionregistrationv1.FailurePolicyType(*failurePolicy)
	return &fp
}
