package admissionpolicy

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
)

func ConvertMatchResources(in *admissionregistrationv1beta1.MatchResources) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		NamespaceSelector:    in.NamespaceSelector,
		ObjectSelector:       in.ObjectSelector,
		MatchPolicy:          (*admissionregistrationv1.MatchPolicyType)(in.MatchPolicy),
		ResourceRules:        convertRules(in.ResourceRules),
		ExcludeResourceRules: convertRules(in.ExcludeResourceRules),
	}
}

func convertRules(rules []admissionregistrationv1beta1.NamedRuleWithOperations) []admissionregistrationv1.NamedRuleWithOperations {
	v1rules := make([]admissionregistrationv1.NamedRuleWithOperations, 0, len(rules))
	for _, r := range rules {
		v1rules = append(v1rules, admissionregistrationv1.NamedRuleWithOperations(r))
	}
	return v1rules
}

func convertMatchConditions(conditions []admissionregistrationv1beta1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(conditions))
	for _, m := range conditions {
		v1conditions = append(v1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1conditions
}

func convertVariables(variables []admissionregistrationv1beta1.Variable) []admissionregistrationv1.Variable {
	v1variables := make([]admissionregistrationv1.Variable, 0, len(variables))
	for _, v := range variables {
		v1variables = append(v1variables, admissionregistrationv1.Variable(v))
	}
	return v1variables
}

func convertParamRef(ref *admissionregistrationv1beta1.ParamRef) *admissionregistrationv1.ParamRef {
	return &admissionregistrationv1.ParamRef{
		Name:                    ref.Name,
		Namespace:               ref.Namespace,
		Selector:                ref.Selector,
		ParameterNotFoundAction: (*admissionregistrationv1.ParameterNotFoundActionType)(ref.ParameterNotFoundAction),
	}
}

func convertParamKind(kind *admissionregistrationv1beta1.ParamKind) *admissionregistrationv1.ParamKind {
	return &admissionregistrationv1.ParamKind{APIVersion: kind.APIVersion, Kind: kind.Kind}
}
