package admissionpolicy

import (
	"crypto/sha256"
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
		// The source policy belongs to the informer cache. Generated expressions must
		// not overwrite it or reuse spare capacity in any of its slices.
		matchConditions = slices.Clone(vpol.Spec.MatchConditions)
		validations = slices.Clone(vpol.Spec.Validations)
		auditAnnotations = slices.Clone(vpol.Spec.AuditAnnotations)
		variables = slices.Clone(vpol.Spec.Variables)

		// Convert CEL exceptions if they exist. Aggregate scoped values so the generated
		// VAP has at most one variable per name, even when several exceptions contribute values.
		var celExceptions []policiesv1beta1.PolicyException
		for _, exception := range exceptions {
			if celpolex := exception.AsCELException(); celpolex != nil {
				celExceptions = append(celExceptions, *celpolex)
			}
		}
		// Informer/list order is not stable. Sort the local copy before aggregating
		// scope literals so repeated reconciliations produce identical CEL.
		slices.SortFunc(celExceptions, func(a, b policiesv1beta1.PolicyException) int {
			if order := strings.Compare(a.Namespace, b.Namespace); order != 0 {
				return order
			}
			return strings.Compare(a.Name, b.Name)
		})
		var exceptionImages, exceptionAllowedValues []string
		for _, exception := range celExceptions {
			exceptionImages = append(exceptionImages, exception.Spec.Images...)
			exceptionAllowedValues = append(exceptionAllowedValues, exception.Spec.AllowedValues...)
		}
		exceptionMatchConditions := ensureUniqueExceptionMatchConditionNames(
			negateExceptionMatchConditions(celExceptions),
			matchConditions,
		)
		matchConditions = append(matchConditions, exceptionMatchConditions...)

		referencesAllowedImages := false
		referencesAllowedValues := false
		var policyExpressions []string
		for _, variable := range variables {
			policyExpressions = append(policyExpressions, variable.Expression)
		}
		for _, validation := range validations {
			policyExpressions = append(policyExpressions, validation.Expression, validation.MessageExpression)
		}
		for _, annotation := range auditAnnotations {
			policyExpressions = append(policyExpressions, annotation.ValueExpression)
		}
		for _, expression := range policyExpressions {
			images, err := replaceExpressions(expression, map[string]string{"exceptions.allowedImages": "[]"})
			if err != nil {
				return err
			}
			values, err := replaceExpressions(expression, map[string]string{"exceptions.allowedValues": "[]"})
			if err != nil {
				return err
			}
			referencesAllowedImages = referencesAllowedImages || images != expression
			referencesAllowedValues = referencesAllowedValues || values != expression
		}

		replacements := map[string]string{}
		matchConditionReplacements := map[string]string{}
		// Native match conditions run before variables are available. Inline the
		// unconditional exception lists here, including an empty list for an absent scope.
		for field, values := range map[string][]string{
			"exceptions.allowedImages": exceptionImages,
			"exceptions.allowedValues": exceptionAllowedValues,
		} {
			if values == nil {
				values = []string{}
			}
			literal, err := celStringListLiteral(values)
			if err != nil {
				return err
			}
			matchConditionReplacements[field] = literal
		}
		for i := range matchConditions {
			expression, err := replaceExpressions(matchConditions[i].Expression, matchConditionReplacements)
			if err != nil {
				return err
			}
			matchConditions[i].Expression = expression
		}
		var exceptionVariables []admissionregistrationv1.Variable
		if len(exceptionImages) > 0 || referencesAllowedImages {
			if exceptionImages == nil {
				exceptionImages = []string{}
			}
			expression, err := celStringListLiteral(exceptionImages)
			if err != nil {
				return err
			}
			availableVariables := append(append([]admissionregistrationv1.Variable{}, variables...), exceptionVariables...)
			name := uniqueVariableName("allowedImages", availableVariables)
			exceptionVariables = append(exceptionVariables, admissionregistrationv1.Variable{
				Name:       name,
				Expression: expression,
			})
			replacements["exceptions.allowedImages"] = "variables." + name
		}
		if len(exceptionAllowedValues) > 0 || referencesAllowedValues {
			if exceptionAllowedValues == nil {
				exceptionAllowedValues = []string{}
			}
			expression, err := celStringListLiteral(exceptionAllowedValues)
			if err != nil {
				return err
			}
			availableVariables := append(append([]admissionregistrationv1.Variable{}, variables...), exceptionVariables...)
			name := uniqueVariableName("allowedValues", availableVariables)
			exceptionVariables = append(exceptionVariables, admissionregistrationv1.Variable{
				Name:       name,
				Expression: expression,
			})
			replacements["exceptions.allowedValues"] = "variables." + name
		}
		for i := range variables {
			expression, err := replaceExpressions(variables[i].Expression, replacements)
			if err != nil {
				return err
			}
			variables[i].Expression = expression
		}
		// Rewrite only policy expressions, then prepend the generated literal lists.
		// Exception-supplied strings must never be rewritten as CEL references.
		// Native VAP variables may only reference earlier declarations.
		variables = append(exceptionVariables, variables...)
		for i := range validations {
			expression, err := replaceExpressions(validations[i].Expression, replacements)
			if err != nil {
				return err
			}
			messageExpression, err := replaceExpressions(validations[i].MessageExpression, replacements)
			if err != nil {
				return err
			}
			validations[i].Expression = expression
			validations[i].MessageExpression = messageExpression
		}
		for i := range auditAnnotations {
			expression, err := replaceExpressions(auditAnnotations[i].ValueExpression, replacements)
			if err != nil {
				return err
			}
			auditAnnotations[i].ValueExpression = expression
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

// negateExceptionMatchConditions returns one negated predicate per exception. A PolicyException
// applies only when all its match conditions evaluate to true, so the generated policy must exclude
// their conjunction as a single condition.
func negateExceptionMatchConditions(exceptions []policiesv1beta1.PolicyException) []admissionregistrationv1.MatchCondition {
	// Informer-backed exception lists have no stable order. Sort a local copy so
	// generated native policy specs do not change across identical reconciliations.
	orderedExceptions := slices.Clone(exceptions)
	slices.SortFunc(orderedExceptions, func(a, b policiesv1beta1.PolicyException) int {
		if byNamespace := strings.Compare(a.Namespace, b.Namespace); byNamespace != 0 {
			return byNamespace
		}
		return strings.Compare(a.Name, b.Name)
	})
	// A global exception with no conditions applies to every request. One false condition
	// is sufficient to disable native policy enforcement, and avoids exceeding Kubernetes'
	// match-condition limit when several such exceptions exist.
	for _, exception := range orderedExceptions {
		if len(exception.Spec.MatchConditions) == 0 &&
			len(exception.Spec.Images) == 0 &&
			len(exception.Spec.AllowedValues) == 0 {
			return []admissionregistrationv1.MatchCondition{{
				Name:       exceptionMatchConditionName(exception),
				Expression: "false",
			}}
		}
	}

	var result []admissionregistrationv1.MatchCondition
	for _, exception := range orderedExceptions {
		conditions := exception.Spec.MatchConditions
		if len(conditions) == 0 {
			// Image/value-scoped exceptions without conditions apply on every request;
			// the generated validation expressions use their aggregated allowed values.
			continue
		}

		expressions := make([]string, len(conditions))
		for i, condition := range conditions {
			expressions[i] = "(" + condition.Expression + ")"
		}
		expression := "!(" + strings.Join(expressions, " && ") + ")"
		if len(conditions) == 1 {
			// Preserve the generated expression for existing single-condition exceptions.
			expression = "!(" + conditions[0].Expression + ")"
		}

		name := conditions[0].Name
		if len(conditions) > 1 {
			// Multiple source conditions become one generated condition. Use a stable short
			// name derived from the exception.
			name = exceptionMatchConditionName(exception)
		}
		result = append(result, admissionregistrationv1.MatchCondition{
			Name:       name,
			Expression: expression,
		})
	}
	return result
}

func exceptionMatchConditionName(exception policiesv1beta1.PolicyException) string {
	sum := sha256.Sum256([]byte(exception.Namespace + "/" + exception.Name))
	return fmt.Sprintf("exception-%x", sum[:6])
}

// ensureUniqueExceptionMatchConditionNames reserves policy-owned names and disambiguates
// exception-generated names, including duplicates between separate exceptions.
func ensureUniqueExceptionMatchConditionNames(
	conditions []admissionregistrationv1.MatchCondition,
	policyConditions []admissionregistrationv1.MatchCondition,
) []admissionregistrationv1.MatchCondition {
	used := make(map[string]struct{}, len(policyConditions)+len(conditions))
	for _, condition := range policyConditions {
		used[condition.Name] = struct{}{}
	}
	for i := range conditions {
		conditions[i].Name = reserveUniqueConditionName(conditions[i].Name, used)
	}
	return conditions
}

func reserveUniqueConditionName(name string, used map[string]struct{}) string {
	if _, exists := used[name]; !exists {
		used[name] = struct{}{}
		return name
	}
	// A match-condition name is a qualified name: only the segment after
	// the optional DNS prefix is limited to 63 characters.
	prefix, segment, qualified := strings.Cut(name, "/")
	if !qualified {
		segment = prefix
	}
	for suffix := 2; ; suffix++ {
		suffixText := fmt.Sprintf("-%d", suffix)
		base := segment
		if len(base)+len(suffixText) > 63 {
			base = base[:63-len(suffixText)]
		}
		candidate := base + suffixText
		if qualified {
			candidate = prefix + "/" + candidate
		}
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

// uniqueVariableName chooses an available CEL variable name for a generated exception variable.
func uniqueVariableName(preferred string, variables []admissionregistrationv1.Variable) string {
	used := make(map[string]struct{}, len(variables))
	for _, variable := range variables {
		used[variable.Name] = struct{}{}
	}
	if _, exists := used[preferred]; !exists {
		return preferred
	}
	base := "exception" + strings.ToUpper(preferred[:1]) + preferred[1:]
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s%d", base, suffix)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

// BuildMutatingAdmissionPolicy is used to build a Kubernetes MutatingAdmissionPolicy from a MutatingPolicy
func BuildMutatingAdmissionPolicy(
	mapol *admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	mp *policiesv1beta1.MutatingPolicy,
	exceptions []policiesv1beta1.PolicyException,
) {
	exceptionMatchConditions := ensureUniqueExceptionMatchConditionNames(
		negateExceptionMatchConditions(exceptions),
		mp.Spec.MatchConditions,
	)
	matchConditions := slicesutils.Map(exceptionMatchConditions, func(mc admissionregistrationv1.MatchCondition) admissionregistrationv1alpha1.MatchCondition {
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
	matchConditions := ensureUniqueExceptionMatchConditionNames(
		negateExceptionMatchConditions(exceptions),
		mp.Spec.MatchConditions,
	)
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
	exceptionMatchConditions := ensureUniqueExceptionMatchConditionNames(
		negateExceptionMatchConditions(exceptions),
		mp.Spec.MatchConditions,
	)
	matchConditions := slicesutils.Map(exceptionMatchConditions, func(mc admissionregistrationv1.MatchCondition) admissionregistrationv1beta1.MatchCondition {
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
