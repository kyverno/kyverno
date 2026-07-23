package admissionpolicy

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConvertMatchResources(in *admissionregistrationv1beta1.MatchResources) *admissionregistrationv1.MatchResources {
	if in == nil {
		return nil
	}
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

func convertRulesToBeta(rules []admissionregistrationv1.NamedRuleWithOperations) []admissionregistrationv1beta1.NamedRuleWithOperations {
	v1beta1Rules := make([]admissionregistrationv1beta1.NamedRuleWithOperations, 0, len(rules))
	for _, r := range rules {
		v1beta1Rules = append(v1beta1Rules, admissionregistrationv1beta1.NamedRuleWithOperations(r))
	}
	return v1beta1Rules
}

func convertMatchConditions(conditions []admissionregistrationv1beta1.MatchCondition) []admissionregistrationv1.MatchCondition {
	v1conditions := make([]admissionregistrationv1.MatchCondition, 0, len(conditions))
	for _, m := range conditions {
		v1conditions = append(v1conditions, admissionregistrationv1.MatchCondition(m))
	}
	return v1conditions
}

func convertMatchConditionsToBeta(conditions []admissionregistrationv1.MatchCondition) []admissionregistrationv1beta1.MatchCondition {
	v1beta1Conditions := make([]admissionregistrationv1beta1.MatchCondition, 0, len(conditions))
	for _, c := range conditions {
		v1beta1Conditions = append(v1beta1Conditions, admissionregistrationv1beta1.MatchCondition(c))
	}
	return v1beta1Conditions
}

func convertVariables(variables []admissionregistrationv1beta1.Variable) []admissionregistrationv1.Variable {
	v1variables := make([]admissionregistrationv1.Variable, 0, len(variables))
	for _, v := range variables {
		v1variables = append(v1variables, admissionregistrationv1.Variable(v))
	}
	return v1variables
}

func convertVariablesToBeta(variables []admissionregistrationv1.Variable) []admissionregistrationv1beta1.Variable {
	v1beta1Variables := make([]admissionregistrationv1beta1.Variable, 0, len(variables))
	for _, v := range variables {
		v1beta1Variables = append(v1beta1Variables, admissionregistrationv1beta1.Variable(v))
	}
	return v1beta1Variables
}

func convertMutations(mutations []admissionregistrationv1beta1.Mutation) []admissionregistrationv1.Mutation {
	v1mutations := make([]admissionregistrationv1.Mutation, 0, len(mutations))
	for _, m := range mutations {
		v1mutations = append(v1mutations, admissionregistrationv1.Mutation{
			PatchType:          admissionregistrationv1.PatchType(m.PatchType),
			ApplyConfiguration: convertApplyConfiguration(m.ApplyConfiguration),
			JSONPatch:          convertJSONPatch(m.JSONPatch),
		})
	}
	return v1mutations
}

func convertMutationsToBeta(mutations []admissionregistrationv1.Mutation) []admissionregistrationv1beta1.Mutation {
	v1beta1Mutations := make([]admissionregistrationv1beta1.Mutation, 0, len(mutations))
	for _, m := range mutations {
		v1beta1Mutations = append(v1beta1Mutations, admissionregistrationv1beta1.Mutation{
			PatchType:          admissionregistrationv1beta1.PatchType(m.PatchType),
			ApplyConfiguration: convertApplyConfigurationToBeta(m.ApplyConfiguration),
			JSONPatch:          convertJSONPatchToBeta(m.JSONPatch),
		})
	}
	return v1beta1Mutations
}

func convertApplyConfiguration(in *admissionregistrationv1beta1.ApplyConfiguration) *admissionregistrationv1.ApplyConfiguration {
	if in == nil {
		return nil
	}
	return &admissionregistrationv1.ApplyConfiguration{Expression: in.Expression}
}

func convertApplyConfigurationToBeta(in *admissionregistrationv1.ApplyConfiguration) *admissionregistrationv1beta1.ApplyConfiguration {
	if in == nil {
		return nil
	}
	return &admissionregistrationv1beta1.ApplyConfiguration{Expression: in.Expression}
}

func convertJSONPatch(in *admissionregistrationv1beta1.JSONPatch) *admissionregistrationv1.JSONPatch {
	if in == nil {
		return nil
	}
	return &admissionregistrationv1.JSONPatch{Expression: in.Expression}
}

func convertJSONPatchToBeta(in *admissionregistrationv1.JSONPatch) *admissionregistrationv1beta1.JSONPatch {
	if in == nil {
		return nil
	}
	return &admissionregistrationv1beta1.JSONPatch{Expression: in.Expression}
}

func convertParamRef(ref *admissionregistrationv1beta1.ParamRef) *admissionregistrationv1.ParamRef {
	if ref == nil {
		return nil
	}
	return &admissionregistrationv1.ParamRef{
		Name:                    ref.Name,
		Namespace:               ref.Namespace,
		Selector:                ref.Selector,
		ParameterNotFoundAction: (*admissionregistrationv1.ParameterNotFoundActionType)(ref.ParameterNotFoundAction),
	}
}

func convertParamRefToBeta(ref *admissionregistrationv1.ParamRef) *admissionregistrationv1beta1.ParamRef {
	if ref == nil {
		return nil
	}
	return &admissionregistrationv1beta1.ParamRef{
		Name:                    ref.Name,
		Namespace:               ref.Namespace,
		Selector:                ref.Selector,
		ParameterNotFoundAction: (*admissionregistrationv1beta1.ParameterNotFoundActionType)(ref.ParameterNotFoundAction),
	}
}

func convertParamKind(kind *admissionregistrationv1beta1.ParamKind) *admissionregistrationv1.ParamKind {
	if kind == nil {
		return nil
	}
	return &admissionregistrationv1.ParamKind{APIVersion: kind.APIVersion, Kind: kind.Kind}
}

func convertParamKindToBeta(kind *admissionregistrationv1.ParamKind) *admissionregistrationv1beta1.ParamKind {
	if kind == nil {
		return nil
	}
	return &admissionregistrationv1beta1.ParamKind{APIVersion: kind.APIVersion, Kind: kind.Kind}
}

func ConvertMutatingAdmissionPolicy(policy *admissionregistrationv1beta1.MutatingAdmissionPolicy) *admissionregistrationv1.MutatingAdmissionPolicy {
	if policy == nil {
		return nil
	}
	return &admissionregistrationv1.MutatingAdmissionPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingAdmissionPolicy",
		},
		ObjectMeta: policy.ObjectMeta,
		Spec: admissionregistrationv1.MutatingAdmissionPolicySpec{
			ParamKind:          convertParamKind(policy.Spec.ParamKind),
			MatchConstraints:   ConvertMatchResources(policy.Spec.MatchConstraints),
			Variables:          convertVariables(policy.Spec.Variables),
			Mutations:          convertMutations(policy.Spec.Mutations),
			FailurePolicy:      (*admissionregistrationv1.FailurePolicyType)(policy.Spec.FailurePolicy),
			MatchConditions:    convertMatchConditions(policy.Spec.MatchConditions),
			ReinvocationPolicy: policy.Spec.ReinvocationPolicy,
		},
	}
}

func ConvertMutatingAdmissionPolicyBinding(binding *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) *admissionregistrationv1.MutatingAdmissionPolicyBinding {
	if binding == nil {
		return nil
	}
	return &admissionregistrationv1.MutatingAdmissionPolicyBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingAdmissionPolicyBinding",
		},
		ObjectMeta: binding.ObjectMeta,
		Spec: admissionregistrationv1.MutatingAdmissionPolicyBindingSpec{
			PolicyName:     binding.Spec.PolicyName,
			ParamRef:       convertParamRef(binding.Spec.ParamRef),
			MatchResources: ConvertMatchResources(binding.Spec.MatchResources),
		},
	}
}

func ConvertMatchResourcesToBeta(in *admissionregistrationv1.MatchResources) *admissionregistrationv1beta1.MatchResources {
	if in == nil {
		return nil
	}
	return &admissionregistrationv1beta1.MatchResources{
		NamespaceSelector:    in.NamespaceSelector,
		ObjectSelector:       in.ObjectSelector,
		MatchPolicy:          (*admissionregistrationv1beta1.MatchPolicyType)(in.MatchPolicy),
		ResourceRules:        convertRulesToBeta(in.ResourceRules),
		ExcludeResourceRules: convertRulesToBeta(in.ExcludeResourceRules),
	}
}

func ConvertMutatingAdmissionPolicyToBeta(policy *admissionregistrationv1.MutatingAdmissionPolicy) *admissionregistrationv1beta1.MutatingAdmissionPolicy {
	if policy == nil {
		return nil
	}
	var failurePolicy *admissionregistrationv1beta1.FailurePolicyType
	if policy.Spec.FailurePolicy != nil {
		converted := admissionregistrationv1beta1.FailurePolicyType(*policy.Spec.FailurePolicy)
		failurePolicy = &converted
	}
	return &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1beta1.SchemeGroupVersion.String(),
			Kind:       "MutatingAdmissionPolicy",
		},
		ObjectMeta: policy.ObjectMeta,
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			ParamKind:          convertParamKindToBeta(policy.Spec.ParamKind),
			MatchConstraints:   ConvertMatchResourcesToBeta(policy.Spec.MatchConstraints),
			Variables:          convertVariablesToBeta(policy.Spec.Variables),
			Mutations:          convertMutationsToBeta(policy.Spec.Mutations),
			FailurePolicy:      failurePolicy,
			MatchConditions:    convertMatchConditionsToBeta(policy.Spec.MatchConditions),
			ReinvocationPolicy: policy.Spec.ReinvocationPolicy,
		},
	}
}

func ConvertMutatingAdmissionPolicyBindingToBeta(binding *admissionregistrationv1.MutatingAdmissionPolicyBinding) *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding {
	if binding == nil {
		return nil
	}
	return &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1beta1.SchemeGroupVersion.String(),
			Kind:       "MutatingAdmissionPolicyBinding",
		},
		ObjectMeta: binding.ObjectMeta,
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
			PolicyName:     binding.Spec.PolicyName,
			ParamRef:       convertParamRefToBeta(binding.Spec.ParamRef),
			MatchResources: ConvertMatchResourcesToBeta(binding.Spec.MatchResources),
		},
	}
}
