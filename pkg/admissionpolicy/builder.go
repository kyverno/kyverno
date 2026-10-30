package admissionpolicy

import (
	"encoding/json"
	"fmt"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	slicesutils "github.com/kyverno/kyverno/pkg/utils/slices"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildValidatingAdmissionPolicy is used to build a Kubernetes ValidatingAdmissionPolicy from a Kyverno policy
func BuildValidatingAdmissionPolicy(
	discoveryClient dclient.IDiscovery,
	vap *admissionregistrationv1.ValidatingAdmissionPolicy,
	policy engineapi.GenericPolicy,
	exceptions []engineapi.GenericException,
) error {
	var matchResources admissionregistrationv1.MatchResources
	var matchConditions []admissionregistrationv1.MatchCondition
	var paramKind *admissionregistrationv1.ParamKind
	var validations []admissionregistrationv1.Validation
	var auditAnnotations []admissionregistrationv1.AuditAnnotation
	var variables []admissionregistrationv1.Variable

	if vpol := policy.AsValidatingPolicy(); vpol != nil {
		matchResources = *vpol.Spec.MatchConstraints
		matchConditions = vpol.Spec.MatchConditions
		validations = vpol.Spec.Validations
		auditAnnotations = vpol.Spec.AuditAnnotations
		variables = vpol.Spec.Variables

		// convert celexceptions if exist
		for _, exception := range exceptions {
			if celpolex := exception.AsCELException(); celpolex != nil {
				for _, matchCondition := range celpolex.Spec.MatchConditions {
					// negate the match condition
					expression := "!(" + matchCondition.Expression + ")"
					matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition{
						Name:       matchCondition.Name,
						Expression: expression,
					})
				}
				if len(celpolex.Spec.Images) > 0 {
					expression, err := celStringListLiteral(celpolex.Spec.Images)
					if err != nil {
						return err
					}
					variables = append(variables, admissionregistrationv1.Variable{
						Name:       "allowedImages",
						Expression: expression,
					})
				}
				if len(celpolex.Spec.AllowedValues) > 0 {
					expression, err := celStringListLiteral(celpolex.Spec.AllowedValues)
					if err != nil {
						return err
					}
					variables = append(variables, admissionregistrationv1.Variable{
						Name:       "allowedValues",
						Expression: expression,
					})
				}
			}
		}
		replacements := map[string]string{
			"exceptions.allowedImages": "variables.allowedImages",
			"exceptions.allowedValues": "variables.allowedValues",
		}
		for i := range validations {
			validations[i].Expression = replaceExpressions(validations[i].Expression, replacements)
		}
	}

	// set owner reference
	vap.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: policy.GetAPIVersion(),
			Kind:       policy.GetKind(),
			Name:       policy.GetName(),
			UID:        policy.GetUID(),
		},
	}
	// set policy spec
	vap.Spec = admissionregistrationv1.ValidatingAdmissionPolicySpec{
		MatchConstraints: &matchResources,
		ParamKind:        paramKind,
		Variables:        variables,
		Validations:      validations,
		AuditAnnotations: auditAnnotations,
		MatchConditions:  matchConditions,
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(vap)
	policyLabels := policy.GetLabels()
	if _, ok := policyLabels[kyverno.LabelExcludeReporting]; ok {
		vap.Labels[kyverno.LabelExcludeReporting] = "true"
	}
	return nil
}

func replaceExpressions(expr string, replacements map[string]string) string {
	for old, new := range replacements {
		expr = strings.ReplaceAll(expr, old, new)
	}
	return expr
}

// BuildValidatingAdmissionPolicyBinding is used to build a Kubernetes ValidatingAdmissionPolicyBinding from a Kyverno policy
func BuildValidatingAdmissionPolicyBinding(
	vapbinding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	policy engineapi.GenericPolicy,
) error {
	var validationActions []admissionregistrationv1.ValidationAction
	var paramRef *admissionregistrationv1.ParamRef
	var policyName string

	if vpol := policy.AsValidatingPolicy(); vpol != nil {
		validationActions = vpol.Spec.ValidationActions()
		policyName = "vpol-" + vpol.GetName()
	}

	// set owner reference
	vapbinding.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: policy.GetAPIVersion(),
			Kind:       policy.GetKind(),
			Name:       policy.GetName(),
			UID:        policy.GetUID(),
		},
	}
	// set binding spec
	vapbinding.Spec = admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
		PolicyName:        policyName,
		ParamRef:          paramRef,
		ValidationActions: validationActions,
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(vapbinding)
	return nil
}

// mutatingPolicyOwnerRef returns the owner reference slice for a MutatingPolicy.
func mutatingPolicyOwnerRef(mp *policiesv1beta1.MutatingPolicy) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: policiesv1beta1.GroupVersion.String(),
			Kind:       mp.GetKind(),
			Name:       mp.GetName(),
			UID:        mp.GetUID(),
		},
	}
}

// negateExceptionMatchConditions returns negated match conditions for each exception,
// using the v1 MatchCondition type which is structurally identical to the alpha/beta variants.
func negateExceptionMatchConditions(exceptions []policiesv1beta1.PolicyException) []admissionregistrationv1.MatchCondition {
	var result []admissionregistrationv1.MatchCondition
	for _, exception := range exceptions {
		for _, mc := range exception.Spec.MatchConditions {
			result = append(result, admissionregistrationv1.MatchCondition{
				Name:       mc.Name,
				Expression: "!(" + mc.Expression + ")",
			})
		}
	}
	return result
}

// BuildMutatingAdmissionPolicy is used to build a Kubernetes MutatingAdmissionPolicy from a MutatingPolicy
func BuildMutatingAdmissionPolicy(
	mapol *admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	mp *policiesv1beta1.MutatingPolicy,
	exceptions []policiesv1beta1.PolicyException,
) {
	matchConditions := slicesutils.Map(negateExceptionMatchConditions(exceptions), func(mc admissionregistrationv1.MatchCondition) admissionregistrationv1alpha1.MatchCondition {
		return admissionregistrationv1alpha1.MatchCondition(mc)
	})
	for _, mc := range mp.Spec.MatchConditions {
		matchConditions = append(matchConditions, admissionregistrationv1alpha1.MatchCondition(mc))
	}

	var fpt *admissionregistrationv1alpha1.FailurePolicyType
	if mp.Spec.FailurePolicy != nil {
		conv := admissionregistrationv1alpha1.FailurePolicyType(*mp.Spec.FailurePolicy)
		fpt = &conv
	}

	// set owner reference
	mapol.OwnerReferences = mutatingPolicyOwnerRef(mp)
	// set policy spec
	mapol.Spec = admissionregistrationv1alpha1.MutatingAdmissionPolicySpec{
		MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
			ResourceRules: slicesutils.Map(mp.Spec.MatchConstraints.ResourceRules, func(rule admissionregistrationv1.NamedRuleWithOperations) admissionregistrationv1alpha1.NamedRuleWithOperations {
				return admissionregistrationv1alpha1.NamedRuleWithOperations{
					ResourceNames:      rule.ResourceNames,
					RuleWithOperations: rule.RuleWithOperations,
				}
			}),
		},
		MatchConditions: matchConditions,
		Mutations:       mp.Spec.Mutations,
		Variables: slicesutils.Map(mp.Spec.Variables, func(v admissionregistrationv1.Variable) admissionregistrationv1alpha1.Variable {
			return admissionregistrationv1alpha1.Variable(v)
		}),
		FailurePolicy:      fpt,
		ReinvocationPolicy: mp.Spec.GetReinvocationPolicy(),
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(mapol)
	policyLabels := mp.GetLabels()
	if _, ok := policyLabels[kyverno.LabelExcludeReporting]; ok {
		mapol.Labels[kyverno.LabelExcludeReporting] = "true"
	}
}

// BuildMutatingAdmissionPolicyBinding is used to build a Kubernetes MutatingAdmissionPolicyBinding from a MutatingPolicy
func BuildMutatingAdmissionPolicyBinding(
	mapbinding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	mp *policiesv1beta1.MutatingPolicy,
) {
	mapbinding.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapbinding.Spec = admissionregistrationv1alpha1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: "mpol-" + mp.GetName(),
	}
	controllerutils.SetManagedByKyvernoLabel(mapbinding)
}

// BuildMutatingAdmissionPolicyV1 is used to build a Kubernetes MutatingAdmissionPolicy (v1) from a MutatingPolicy.
func BuildMutatingAdmissionPolicyV1(
	mapol *admissionregistrationv1.MutatingAdmissionPolicy,
	mp *policiesv1beta1.MutatingPolicy,
	exceptions []policiesv1beta1.PolicyException,
) {
	matchConditions := negateExceptionMatchConditions(exceptions)
	for _, mc := range mp.Spec.MatchConditions {
		matchConditions = append(matchConditions, mc)
	}

	var fpt *admissionregistrationv1.FailurePolicyType
	if mp.Spec.FailurePolicy != nil {
		conv := *mp.Spec.FailurePolicy
		fpt = &conv
	}

	mapol.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapol.Spec = admissionregistrationv1.MutatingAdmissionPolicySpec{
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: slicesutils.Map(mp.Spec.MatchConstraints.ResourceRules, func(rule admissionregistrationv1.NamedRuleWithOperations) admissionregistrationv1.NamedRuleWithOperations {
				return admissionregistrationv1.NamedRuleWithOperations{
					ResourceNames:      rule.ResourceNames,
					RuleWithOperations: rule.RuleWithOperations,
				}
			}),
		},
		MatchConditions: matchConditions,
		Mutations: slicesutils.Map(mp.Spec.Mutations, func(m admissionregistrationv1alpha1.Mutation) admissionregistrationv1.Mutation {
			mut := admissionregistrationv1.Mutation{PatchType: admissionregistrationv1.PatchType(m.PatchType)}
			if m.ApplyConfiguration != nil {
				mut.ApplyConfiguration = &admissionregistrationv1.ApplyConfiguration{Expression: m.ApplyConfiguration.Expression}
			}
			if m.JSONPatch != nil {
				mut.JSONPatch = &admissionregistrationv1.JSONPatch{Expression: m.JSONPatch.Expression}
			}
			return mut
		}),
		Variables: slicesutils.Map(mp.Spec.Variables, func(v admissionregistrationv1.Variable) admissionregistrationv1.Variable {
			return v
		}),
		FailurePolicy:      fpt,
		ReinvocationPolicy: mp.Spec.GetReinvocationPolicy(),
	}
	controllerutils.SetManagedByKyvernoLabel(mapol)
	policyLabels := mp.GetLabels()
	if _, ok := policyLabels[kyverno.LabelExcludeReporting]; ok {
		mapol.Labels[kyverno.LabelExcludeReporting] = "true"
	}
}

// BuildMutatingAdmissionPolicyBindingV1 is used to build a Kubernetes MutatingAdmissionPolicyBinding (v1) from a MutatingPolicy.
func BuildMutatingAdmissionPolicyBindingV1(
	mapbinding *admissionregistrationv1.MutatingAdmissionPolicyBinding,
	mp *policiesv1beta1.MutatingPolicy,
) {
	mapbinding.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapbinding.Spec = admissionregistrationv1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: "mpol-" + mp.GetName(),
	}
	controllerutils.SetManagedByKyvernoLabel(mapbinding)
}

// BuildMutatingAdmissionPolicyBeta is used to build a Kubernetes MutatingAdmissionPolicy (v1beta1) from a MutatingPolicy
func BuildMutatingAdmissionPolicyBeta(
	mapol *admissionregistrationv1beta1.MutatingAdmissionPolicy,
	mp *policiesv1beta1.MutatingPolicy,
	exceptions []policiesv1beta1.PolicyException,
) {
	matchConditions := slicesutils.Map(negateExceptionMatchConditions(exceptions), func(mc admissionregistrationv1.MatchCondition) admissionregistrationv1beta1.MatchCondition {
		return admissionregistrationv1beta1.MatchCondition(mc)
	})
	for _, mc := range mp.Spec.MatchConditions {
		matchConditions = append(matchConditions, admissionregistrationv1beta1.MatchCondition(mc))
	}

	var fpt *admissionregistrationv1beta1.FailurePolicyType
	if mp.Spec.FailurePolicy != nil {
		conv := admissionregistrationv1beta1.FailurePolicyType(*mp.Spec.FailurePolicy)
		fpt = &conv
	}

	// set owner reference
	mapol.OwnerReferences = mutatingPolicyOwnerRef(mp)
	// set policy spec
	mapol.Spec = admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
		MatchConstraints: &admissionregistrationv1beta1.MatchResources{
			ResourceRules: slicesutils.Map(mp.Spec.MatchConstraints.ResourceRules, func(rule admissionregistrationv1.NamedRuleWithOperations) admissionregistrationv1beta1.NamedRuleWithOperations {
				return admissionregistrationv1beta1.NamedRuleWithOperations{
					ResourceNames:      rule.ResourceNames,
					RuleWithOperations: rule.RuleWithOperations,
				}
			}),
		},
		MatchConditions: matchConditions,
		Mutations: slicesutils.Map(mp.Spec.Mutations, func(m admissionregistrationv1alpha1.Mutation) admissionregistrationv1beta1.Mutation {
			mut := admissionregistrationv1beta1.Mutation{
				PatchType: admissionregistrationv1beta1.PatchType(m.PatchType),
			}
			if m.ApplyConfiguration != nil {
				mut.ApplyConfiguration = &admissionregistrationv1beta1.ApplyConfiguration{
					Expression: m.ApplyConfiguration.Expression,
				}
			}
			if m.JSONPatch != nil {
				mut.JSONPatch = &admissionregistrationv1beta1.JSONPatch{
					Expression: m.JSONPatch.Expression,
				}
			}
			return mut
		}),
		Variables: slicesutils.Map(mp.Spec.Variables, func(v admissionregistrationv1.Variable) admissionregistrationv1beta1.Variable {
			return admissionregistrationv1beta1.Variable(v)
		}),
		FailurePolicy:      fpt,
		ReinvocationPolicy: mp.Spec.GetReinvocationPolicy(),
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(mapol)
	policyLabels := mp.GetLabels()
	if _, ok := policyLabels[kyverno.LabelExcludeReporting]; ok {
		mapol.Labels[kyverno.LabelExcludeReporting] = "true"
	}
}

// BuildMutatingAdmissionPolicyBindingBeta is used to build a Kubernetes MutatingAdmissionPolicyBinding (v1beta1) from a MutatingPolicy
func BuildMutatingAdmissionPolicyBindingBeta(
	mapbinding *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding,
	mp *policiesv1beta1.MutatingPolicy,
) {
	mapbinding.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapbinding.Spec = admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: "mpol-" + mp.GetName(),
	}
	controllerutils.SetManagedByKyvernoLabel(mapbinding)
}

// celStringListLiteral encodes values as a CEL list-of-strings literal.
//
// A JSON array of strings is also a valid CEL list literal, and the JSON encoder
// escapes quotes and backslashes, so an exception-supplied value stays a string
// instead of breaking out of the literal and becoming expression code.
func celStringListLiteral(values []string) (string, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("failed to encode %v as a CEL list literal: %w", values, err)
	}
	return string(encoded), nil
}
