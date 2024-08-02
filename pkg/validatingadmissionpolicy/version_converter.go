package validatingadmissionpolicy

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConvertValidatingAdmissionPolicy is used to convert v1alpha1 of ValidatingAdmissionPolicy to v1beta1
func ConvertValidatingAdmissionPolicy(v1alpha1policy v1alpha1.ValidatingAdmissionPolicy) v1beta1.ValidatingAdmissionPolicy {
	var namespaceSelector, objectSelector metav1.LabelSelector
	if v1alpha1policy.Spec.MatchConstraints.NamespaceSelector != nil {
		namespaceSelector = *v1alpha1policy.Spec.MatchConstraints.NamespaceSelector
	}
	if v1alpha1policy.Spec.MatchConstraints.ObjectSelector != nil {
		objectSelector = *v1alpha1policy.Spec.MatchConstraints.ObjectSelector
	}
	v1beta1policy := v1beta1.ValidatingAdmissionPolicy{
		Spec: v1beta1.ValidatingAdmissionPolicySpec{
			FailurePolicy: (*v1beta1.FailurePolicyType)(v1alpha1policy.Spec.FailurePolicy),
			ParamKind:     (*v1beta1.ParamKind)(v1alpha1policy.Spec.ParamKind),
			MatchConstraints: &v1beta1.MatchResources{
				NamespaceSelector:    &namespaceSelector,
				ObjectSelector:       &objectSelector,
				ResourceRules:        convertRules(v1alpha1policy.Spec.MatchConstraints.ResourceRules),
				ExcludeResourceRules: convertRules(v1alpha1policy.Spec.MatchConstraints.ExcludeResourceRules),
				MatchPolicy:          (*v1beta1.MatchPolicyType)(v1alpha1policy.Spec.MatchConstraints.MatchPolicy),
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
func ConvertValidatingAdmissionPolicyBinding(v1alpha1binding v1alpha1.ValidatingAdmissionPolicyBinding) v1beta1.ValidatingAdmissionPolicyBinding {
	var namespaceSelector, objectSelector, paramSelector metav1.LabelSelector
	var resourceRules, excludeResourceRules []v1alpha1.NamedRuleWithOperations
	var matchPolicy *v1alpha1.MatchPolicyType
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

	var paramRef v1beta1.ParamRef
	if v1alpha1binding.Spec.ParamRef != nil {
		paramRef.Name = v1alpha1binding.Spec.ParamRef.Name
		paramRef.Namespace = v1alpha1binding.Spec.ParamRef.Namespace
		if v1alpha1binding.Spec.ParamRef.Selector != nil {
			paramRef.Selector = v1alpha1binding.Spec.ParamRef.Selector
		} else {
			paramRef.Selector = &paramSelector
		}
		paramRef.ParameterNotFoundAction = (*v1beta1.ParameterNotFoundActionType)(v1alpha1binding.Spec.ParamRef.ParameterNotFoundAction)
	}

	v1beta1binding := v1beta1.ValidatingAdmissionPolicyBinding{
		Spec: v1beta1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: v1alpha1binding.Spec.PolicyName,
			ParamRef:   &paramRef,
			MatchResources: &v1beta1.MatchResources{
				NamespaceSelector:    &namespaceSelector,
				ObjectSelector:       &objectSelector,
				ResourceRules:        convertRules(resourceRules),
				ExcludeResourceRules: convertRules(excludeResourceRules),
				MatchPolicy:          (*v1beta1.MatchPolicyType)(matchPolicy),
			},
			ValidationActions: convertValidationActions(v1alpha1binding.Spec.ValidationActions),
		},
	}
	return v1beta1binding
}

func convertRules(v1alpha1rules []v1alpha1.NamedRuleWithOperations) []v1beta1.NamedRuleWithOperations {
	v1beta1rules := make([]v1beta1.NamedRuleWithOperations, 0, len(v1alpha1rules))
	for _, r := range v1alpha1rules {
		v1beta1rules = append(v1beta1rules, v1beta1.NamedRuleWithOperations(r))
	}
	return v1beta1rules
}

func convertValidations(v1alpha1validations []v1alpha1.Validation) []v1beta1.Validation {
	v1beta1validations := make([]v1beta1.Validation, 0, len(v1alpha1validations))
	for _, v := range v1alpha1validations {
		v1beta1validations = append(v1beta1validations, v1beta1.Validation(v))
	}
	return v1beta1validations
}

func convertAuditAnnotations(v1alpha1auditanns []v1alpha1.AuditAnnotation) []v1beta1.AuditAnnotation {
	v1beta1auditanns := make([]v1beta1.AuditAnnotation, 0, len(v1alpha1auditanns))
	for _, a := range v1alpha1auditanns {
		v1beta1auditanns = append(v1beta1auditanns, v1beta1.AuditAnnotation(a))
	}
	return v1beta1auditanns
}

func convertMatchConditions(v1alpha1conditions []v1alpha1.MatchCondition) []v1beta1.MatchCondition {
	v1beta1conditions := make([]v1beta1.MatchCondition, 0, len(v1alpha1conditions))
	for _, m := range v1alpha1conditions {
		v1beta1conditions = append(v1beta1conditions, v1beta1.MatchCondition(m))
	}
	return v1beta1conditions
}

func convertVariables(v1alpha1variables []v1alpha1.Variable) []v1beta1.Variable {
	v1beta1variables := make([]v1beta1.Variable, 0, len(v1alpha1variables))
	for _, v := range v1alpha1variables {
		v1beta1variables = append(v1beta1variables, v1beta1.Variable(v))
	}
	return v1beta1variables
}

func convertValidationActions(v1alpha1actions []v1alpha1.ValidationAction) []v1beta1.ValidationAction {
	v1beta1actions := make([]v1beta1.ValidationAction, 0, len(v1alpha1actions))
	for _, a := range v1alpha1actions {
		v1beta1actions = append(v1beta1actions, v1beta1.ValidationAction(a))
	}
	return v1beta1actions
}

func ConvertMatchConditionsV1(v1alpha1conditions []v1alpha1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(v1alpha1conditions))
	for _, m := range v1alpha1conditions {
		v1conditions = append(v1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1conditions
}
