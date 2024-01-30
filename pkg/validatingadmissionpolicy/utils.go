package validatingadmissionpolicy

import (
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/api/admissionregistration/v1beta1"
)

func convertRules(v1alpha1rules []v1alpha1.NamedRuleWithOperations) []v1beta1.NamedRuleWithOperations {
	var v1beta1rules []v1beta1.NamedRuleWithOperations
	for _, r := range v1alpha1rules {
		v1beta1rules = append(v1beta1rules, v1beta1.NamedRuleWithOperations(r))
	}
	return v1beta1rules
}

func convertValidations(v1alpha1validations []v1alpha1.Validation) []v1beta1.Validation {
	var v1beta1validations []v1beta1.Validation
	for _, v := range v1alpha1validations {
		v1beta1validations = append(v1beta1validations, v1beta1.Validation(v))
	}
	return v1beta1validations
}

func convertAuditAnnotations(v1alpha1auditanns []v1alpha1.AuditAnnotation) []v1beta1.AuditAnnotation {
	var v1beta1auditanns []v1beta1.AuditAnnotation
	for _, a := range v1alpha1auditanns {
		v1beta1auditanns = append(v1beta1auditanns, v1beta1.AuditAnnotation(a))
	}
	return v1beta1auditanns
}

func convertMatchConditions(v1alpha1conditions []v1alpha1.MatchCondition) []v1beta1.MatchCondition {
	var v1beta1conditions []v1beta1.MatchCondition
	for _, m := range v1alpha1conditions {
		v1beta1conditions = append(v1beta1conditions, v1beta1.MatchCondition(m))
	}
	return v1beta1conditions
}

func convertVariables(v1alpha1variables []v1alpha1.Variable) []v1beta1.Variable {
	var v1beta1variables []v1beta1.Variable
	for _, v := range v1alpha1variables {
		v1beta1variables = append(v1beta1variables, v1beta1.Variable(v))
	}
	return v1beta1variables
}

func convertValidationActions(v1alpha1actions []v1alpha1.ValidationAction) []v1beta1.ValidationAction {
	var v1beta1actions []v1beta1.ValidationAction
	for _, a := range v1alpha1actions {
		v1beta1actions = append(v1beta1actions, v1beta1.ValidationAction(a))
	}
	return v1beta1actions
}
