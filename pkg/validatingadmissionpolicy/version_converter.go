package validatingadmissionpolicy

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConvertValidatingAdmissionPolicy is used to convert v1alpha1 of ValidatingAdmissionPolicy to v1
func ConvertValidatingAdmissionPolicy(v1alpha1policy admissionregistrationv1alpha1.ValidatingAdmissionPolicy) admissionregistrationv1.ValidatingAdmissionPolicy {
	var namespaceSelector, objectSelector metav1.LabelSelector
	if v1alpha1policy.Spec.MatchConstraints.NamespaceSelector != nil {
		namespaceSelector = *v1alpha1policy.Spec.MatchConstraints.NamespaceSelector
	}
	if v1alpha1policy.Spec.MatchConstraints.ObjectSelector != nil {
		objectSelector = *v1alpha1policy.Spec.MatchConstraints.ObjectSelector
	}
	v1beta1policy := admissionregistrationv1.ValidatingAdmissionPolicy{
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: (*admissionregistrationv1.FailurePolicyType)(v1alpha1policy.Spec.FailurePolicy),
			ParamKind:     (*admissionregistrationv1.ParamKind)(v1alpha1policy.Spec.ParamKind),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector:    &namespaceSelector,
				ObjectSelector:       &objectSelector,
				ResourceRules:        convertRules(v1alpha1policy.Spec.MatchConstraints.ResourceRules),
				ExcludeResourceRules: convertRules(v1alpha1policy.Spec.MatchConstraints.ExcludeResourceRules),
				MatchPolicy:          (*admissionregistrationv1.MatchPolicyType)(v1alpha1policy.Spec.MatchConstraints.MatchPolicy),
			},
			Validations:      convertValidations(v1alpha1policy.Spec.Validations),
			AuditAnnotations: convertAuditAnnotations(v1alpha1policy.Spec.AuditAnnotations),
			MatchConditions:  convertMatchConditions(v1alpha1policy.Spec.MatchConditions),
			Variables:        convertVariables(v1alpha1policy.Spec.Variables),
		},
	}
	return v1beta1policy
}

// ConvertValidatingAdmissionPolicyBinding is used to convert v1alpha1 of ValidatingAdmissionPolicyBinding to v1beta1
func ConvertValidatingAdmissionPolicyBinding(v1alpha1binding admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding) admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	var namespaceSelector, objectSelector, paramSelector metav1.LabelSelector
	var resourceRules, excludeResourceRules []admissionregistrationv1alpha1.NamedRuleWithOperations
	var matchPolicy *admissionregistrationv1alpha1.MatchPolicyType
	if v1alpha1binding.Spec.MatchResources != nil {
		if v1alpha1binding.Spec.MatchResources.NamespaceSelector != nil {
			namespaceSelector = *v1alpha1binding.Spec.MatchResources.NamespaceSelector
		}
		if v1alpha1binding.Spec.MatchResources.ObjectSelector != nil {
			objectSelector = *v1alpha1binding.Spec.MatchResources.ObjectSelector
		}
		resourceRules = v1alpha1binding.Spec.MatchResources.ResourceRules
		excludeResourceRules = v1alpha1binding.Spec.MatchResources.ExcludeResourceRules
		matchPolicy = v1alpha1binding.Spec.MatchResources.MatchPolicy
	}

	var paramRef admissionregistrationv1.ParamRef
	if v1alpha1binding.Spec.ParamRef != nil {
		paramRef.Name = v1alpha1binding.Spec.ParamRef.Name
		paramRef.Namespace = v1alpha1binding.Spec.ParamRef.Namespace
		if v1alpha1binding.Spec.ParamRef.Selector != nil {
			paramRef.Selector = v1alpha1binding.Spec.ParamRef.Selector
		} else {
			paramRef.Selector = &paramSelector
		}
		paramRef.ParameterNotFoundAction = (*admissionregistrationv1.ParameterNotFoundActionType)(v1alpha1binding.Spec.ParamRef.ParameterNotFoundAction)
	}

	v1beta1binding := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: v1alpha1binding.Spec.PolicyName,
			ParamRef:   &paramRef,
			MatchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector:    &namespaceSelector,
				ObjectSelector:       &objectSelector,
				ResourceRules:        convertRules(resourceRules),
				ExcludeResourceRules: convertRules(excludeResourceRules),
				MatchPolicy:          (*admissionregistrationv1.MatchPolicyType)(matchPolicy),
			},
			ValidationActions: convertValidationActions(v1alpha1binding.Spec.ValidationActions),
		},
	}
	return v1beta1binding
}

func convertRules(v1alpha1rules []admissionregistrationv1alpha1.NamedRuleWithOperations) []admissionregistrationv1.NamedRuleWithOperations {
	v1beta1rules := make([]admissionregistrationv1.NamedRuleWithOperations, 0, len(v1alpha1rules))
	for _, r := range v1alpha1rules {
		v1beta1rules = append(v1beta1rules, admissionregistrationv1.NamedRuleWithOperations(r))
	}
	return v1beta1rules
}

func convertValidations(v1alpha1validations []admissionregistrationv1alpha1.Validation) []admissionregistrationv1.Validation {
	v1beta1validations := make([]admissionregistrationv1.Validation, 0, len(v1alpha1validations))
	for _, v := range v1alpha1validations {
		v1beta1validations = append(v1beta1validations, admissionregistrationv1.Validation(v))
	}
	return v1beta1validations
}

func convertAuditAnnotations(v1alpha1auditanns []admissionregistrationv1alpha1.AuditAnnotation) []admissionregistrationv1.AuditAnnotation {
	v1beta1auditanns := make([]admissionregistrationv1.AuditAnnotation, 0, len(v1alpha1auditanns))
	for _, a := range v1alpha1auditanns {
		v1beta1auditanns = append(v1beta1auditanns, admissionregistrationv1.AuditAnnotation(a))
	}
	return v1beta1auditanns
}

func convertMatchConditions(v1alpha1conditions []admissionregistrationv1alpha1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1beta1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(v1alpha1conditions))
	for _, m := range v1alpha1conditions {
		v1beta1conditions = append(v1beta1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1beta1conditions
}

func convertVariables(v1alpha1variables []admissionregistrationv1alpha1.Variable) []admissionregistrationv1.Variable {
	v1beta1variables := make([]admissionregistrationv1.Variable, 0, len(v1alpha1variables))
	for _, v := range v1alpha1variables {
		v1beta1variables = append(v1beta1variables, admissionregistrationv1.Variable(v))
	}
	return v1beta1variables
}

func convertValidationActions(v1alpha1actions []admissionregistrationv1alpha1.ValidationAction) []admissionregistrationv1.ValidationAction {
	v1beta1actions := make([]admissionregistrationv1.ValidationAction, 0, len(v1alpha1actions))
	for _, a := range v1alpha1actions {
		v1beta1actions = append(v1beta1actions, admissionregistrationv1.ValidationAction(a))
	}
	return v1beta1actions
}

func ConvertMatchConditionsV1(v1alpha1conditions []admissionregistrationv1alpha1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(v1alpha1conditions))
	for _, m := range v1alpha1conditions {
		v1conditions = append(v1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1conditions
}
