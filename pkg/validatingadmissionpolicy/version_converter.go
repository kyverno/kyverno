package validatingadmissionpolicy

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConvertValidatingAdmissionPolicy is used to convert v1beta1 of ValidatingAdmissionPolicy to v1
func ConvertValidatingAdmissionPolicy(v1beta1policy admissionregistrationv1beta1.ValidatingAdmissionPolicy) admissionregistrationv1.ValidatingAdmissionPolicy {
	var namespaceSelector, objectSelector metav1.LabelSelector
	if v1beta1policy.Spec.MatchConstraints.NamespaceSelector != nil {
		namespaceSelector = *v1beta1policy.Spec.MatchConstraints.NamespaceSelector
	}
	if v1beta1policy.Spec.MatchConstraints.ObjectSelector != nil {
		objectSelector = *v1beta1policy.Spec.MatchConstraints.ObjectSelector
	}
	v1policy := admissionregistrationv1.ValidatingAdmissionPolicy{
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: (*admissionregistrationv1.FailurePolicyType)(v1beta1policy.Spec.FailurePolicy),
			ParamKind:     (*admissionregistrationv1.ParamKind)(v1beta1policy.Spec.ParamKind),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector:    &namespaceSelector,
				ObjectSelector:       &objectSelector,
				ResourceRules:        convertRules(v1beta1policy.Spec.MatchConstraints.ResourceRules),
				ExcludeResourceRules: convertRules(v1beta1policy.Spec.MatchConstraints.ExcludeResourceRules),
				MatchPolicy:          (*admissionregistrationv1.MatchPolicyType)(v1beta1policy.Spec.MatchConstraints.MatchPolicy),
			},
			Validations:      convertValidations(v1beta1policy.Spec.Validations),
			AuditAnnotations: convertAuditAnnotations(v1beta1policy.Spec.AuditAnnotations),
			MatchConditions:  convertMatchConditions(v1beta1policy.Spec.MatchConditions),
			Variables:        convertVariables(v1beta1policy.Spec.Variables),
		},
	}
	return v1policy
}

// ConvertValidatingAdmissionPolicyBinding is used to convert v1beta1 of ValidatingAdmissionPolicyBinding to v1.
func ConvertValidatingAdmissionPolicyBinding(v1beta1binding admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	var namespaceSelector, objectSelector, paramSelector metav1.LabelSelector
	var resourceRules, excludeResourceRules []admissionregistrationv1beta1.NamedRuleWithOperations
	var matchPolicy *admissionregistrationv1beta1.MatchPolicyType
	if v1beta1binding.Spec.MatchResources != nil {
		if v1beta1binding.Spec.MatchResources.NamespaceSelector != nil {
			namespaceSelector = *v1beta1binding.Spec.MatchResources.NamespaceSelector
		}
		if v1beta1binding.Spec.MatchResources.ObjectSelector != nil {
			objectSelector = *v1beta1binding.Spec.MatchResources.ObjectSelector
		}
		resourceRules = v1beta1binding.Spec.MatchResources.ResourceRules
		excludeResourceRules = v1beta1binding.Spec.MatchResources.ExcludeResourceRules
		matchPolicy = v1beta1binding.Spec.MatchResources.MatchPolicy
	}

	var paramRef admissionregistrationv1.ParamRef
	if v1beta1binding.Spec.ParamRef != nil {
		paramRef.Name = v1beta1binding.Spec.ParamRef.Name
		paramRef.Namespace = v1beta1binding.Spec.ParamRef.Namespace
		if v1beta1binding.Spec.ParamRef.Selector != nil {
			paramRef.Selector = v1beta1binding.Spec.ParamRef.Selector
		} else {
			paramRef.Selector = &paramSelector
		}
		paramRef.ParameterNotFoundAction = (*admissionregistrationv1.ParameterNotFoundActionType)(v1beta1binding.Spec.ParamRef.ParameterNotFoundAction)
	}

	v1binding := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: v1beta1binding.Spec.PolicyName,
			ParamRef:   &paramRef,
			MatchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector:    &namespaceSelector,
				ObjectSelector:       &objectSelector,
				ResourceRules:        convertRules(resourceRules),
				ExcludeResourceRules: convertRules(excludeResourceRules),
				MatchPolicy:          (*admissionregistrationv1.MatchPolicyType)(matchPolicy),
			},
			ValidationActions: convertValidationActions(v1beta1binding.Spec.ValidationActions),
		},
	}
	return v1binding
}

func convertRules(v1beta1rules []admissionregistrationv1beta1.NamedRuleWithOperations) []admissionregistrationv1.NamedRuleWithOperations {
	v1rules := make([]admissionregistrationv1.NamedRuleWithOperations, 0, len(v1beta1rules))
	for _, r := range v1beta1rules {
		v1rules = append(v1rules, admissionregistrationv1.NamedRuleWithOperations(r))
	}
	return v1rules
}

func convertValidations(v1beta1validations []admissionregistrationv1beta1.Validation) []admissionregistrationv1.Validation {
	v1validations := make([]admissionregistrationv1.Validation, 0, len(v1beta1validations))
	for _, v := range v1beta1validations {
		v1validations = append(v1validations, admissionregistrationv1.Validation(v))
	}
	return v1validations
}

func convertAuditAnnotations(v1beta1auditanns []admissionregistrationv1beta1.AuditAnnotation) []admissionregistrationv1.AuditAnnotation {
	v1auditanns := make([]admissionregistrationv1.AuditAnnotation, 0, len(v1beta1auditanns))
	for _, a := range v1beta1auditanns {
		v1auditanns = append(v1auditanns, admissionregistrationv1.AuditAnnotation(a))
	}
	return v1auditanns
}

func convertMatchConditions(v1beta1conditions []admissionregistrationv1beta1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(v1beta1conditions))
	for _, m := range v1beta1conditions {
		v1conditions = append(v1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1conditions
}

func convertVariables(v1beta1variables []admissionregistrationv1beta1.Variable) []admissionregistrationv1.Variable {
	v1variables := make([]admissionregistrationv1.Variable, 0, len(v1beta1variables))
	for _, v := range v1beta1variables {
		v1variables = append(v1variables, admissionregistrationv1.Variable(v))
	}
	return v1variables
}

func convertValidationActions(v1beta1actions []admissionregistrationv1beta1.ValidationAction) []admissionregistrationv1.ValidationAction {
	v1actions := make([]admissionregistrationv1.ValidationAction, 0, len(v1beta1actions))
	for _, a := range v1beta1actions {
		v1actions = append(v1actions, admissionregistrationv1.ValidationAction(a))
	}
	return v1actions
}

func ConvertMatchConditionsV1(v1beta1conditions []admissionregistrationv1beta1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(v1beta1conditions))
	for _, m := range v1beta1conditions {
		v1conditions = append(v1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1conditions
}
