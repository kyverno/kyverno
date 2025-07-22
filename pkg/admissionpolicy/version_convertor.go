package admissionpolicy

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
)

func ConvertMatchResources(in *admissionregistrationv1alpha1.MatchResources) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		NamespaceSelector:    in.NamespaceSelector,
		ObjectSelector:       in.ObjectSelector,
		MatchPolicy:          (*admissionregistrationv1.MatchPolicyType)(in.MatchPolicy),
		ResourceRules:        convertRules(in.ResourceRules),
		ExcludeResourceRules: convertRules(in.ExcludeResourceRules),
	}
}

func convertRules(v1alpha1rules []admissionregistrationv1alpha1.NamedRuleWithOperations) []admissionregistrationv1.NamedRuleWithOperations {
	v1rules := make([]admissionregistrationv1.NamedRuleWithOperations, 0, len(v1alpha1rules))
	for _, r := range v1alpha1rules {
		v1rules = append(v1rules, admissionregistrationv1.NamedRuleWithOperations(r))
	}
	return v1rules
}

func convertMatchConditions(v1alpha1conditions []admissionregistrationv1alpha1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(v1alpha1conditions))
	for _, m := range v1alpha1conditions {
		v1conditions = append(v1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1conditions
}

func convertVariables(v1alpha1variables []admissionregistrationv1alpha1.Variable) []admissionregistrationv1.Variable {
	v1variables := make([]admissionregistrationv1.Variable, 0, len(v1alpha1variables))
	for _, v := range v1alpha1variables {
		v1variables = append(v1variables, admissionregistrationv1.Variable(v))
	}
	return v1variables
}
