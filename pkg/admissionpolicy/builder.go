package admissionpolicy

import (
	"fmt"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	mpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/mpol/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	slicesutils "github.com/kyverno/kyverno/pkg/utils/slices"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildValidatingAdmissionPolicy is used to build a Kubernetes ValidatingAdmissionPolicy from a Kyverno policy.
// specOverride, when non-nil, is used in place of the ValidatingPolicy's own spec - this is how an
// autogen'd variant (e.g. the "defaults"/"cronjobs" rewritten spec from policy.GetStatus().Autogen.Configs)
// is turned into its own ValidatingAdmissionPolicy while owner reference and labels still derive from
// the real policy. group is the matching autogen.ReplacementsMap key ("" for the base/non-autogen'd
// variant) - it's what lets a PolicyException's match condition, written against the Pod-shaped base
// object, still work once embedded into an autogen-group VAP whose object is a Deployment/CronJob: the
// same object.spec-&gt;object.spec.template.spec (etc.) rewrite validations already get is applied to the
// exception's expression too, using group to look up the right autogen.ReplacementsMap entry. Without
// this, an exception expression that assumes the Pod shape (e.g. object.spec.containers...) would
// silently never match, or error, once applied to a Deployment. Only consulted on the ValidatingPolicy
// path; ClusterPolicy ignores both parameters.
func BuildValidatingAdmissionPolicy(
	discoveryClient dclient.IDiscovery,
	vap *admissionregistrationv1.ValidatingAdmissionPolicy,
	policy engineapi.GenericPolicy,
	exceptions []engineapi.GenericException,
	group string,
	specOverride *policiesv1beta1.ValidatingPolicySpec,
) error {
	var matchResources admissionregistrationv1.MatchResources
	var matchConditions []admissionregistrationv1.MatchCondition
	var paramKind *admissionregistrationv1.ParamKind
	var validations []admissionregistrationv1.Validation
	var auditAnnotations []admissionregistrationv1.AuditAnnotation
	var variables []admissionregistrationv1.Variable

	if cpol := policy.AsKyvernoPolicy(); cpol != nil {
		// construct the rules
		var matchRules, excludeRules []admissionregistrationv1.NamedRuleWithOperations

		rule := cpol.GetSpec().Rules[0]

		// convert the match block
		match := rule.MatchResources
		if !match.ResourceDescription.IsEmpty() {
			if err := translateResource(discoveryClient, &matchResources, &matchRules, match.ResourceDescription, true); err != nil {
				return err
			}
		}

		if match.Any != nil {
			if err := translateResourceFilters(discoveryClient, &matchResources, &matchRules, match.Any, true); err != nil {
				return err
			}
		}
		if match.All != nil {
			if err := translateResourceFilters(discoveryClient, &matchResources, &matchRules, match.All, true); err != nil {
				return err
			}
		}

		// convert the exclude block
		if exclude := rule.ExcludeResources; exclude != nil {
			if !exclude.ResourceDescription.IsEmpty() {
				if err := translateResource(discoveryClient, &matchResources, &excludeRules, exclude.ResourceDescription, false); err != nil {
					return err
				}
			}

			if exclude.Any != nil {
				if err := translateResourceFilters(discoveryClient, &matchResources, &excludeRules, exclude.Any, false); err != nil {
					return err
				}
			}
			if exclude.All != nil {
				if err := translateResourceFilters(discoveryClient, &matchResources, &excludeRules, exclude.All, false); err != nil {
					return err
				}
			}
		}

		// convert the exceptions if exist
		for _, exception := range exceptions {
			if polex := exception.AsException(); polex != nil {
				match := polex.Spec.Match
				if match.Any != nil {
					if err := translateResourceFilters(discoveryClient, &matchResources, &excludeRules, match.Any, false); err != nil {
						return err
					}
				}

				if match.All != nil {
					if err := translateResourceFilters(discoveryClient, &matchResources, &excludeRules, match.All, false); err != nil {
						return err
					}
				}
			}
		}

		matchConditions = rule.CELPreconditions
		paramKind = rule.Validation.CEL.ParamKind
		validations = rule.Validation.CEL.Expressions
		auditAnnotations = rule.Validation.CEL.AuditAnnotations
		variables = rule.Validation.CEL.Variables
	} else if vpol := policy.AsValidatingPolicy(); vpol != nil {
		spec := vpol.Spec
		if specOverride != nil {
			spec = *specOverride
		}
		matchResources = *spec.MatchConstraints
		matchConditions = spec.MatchConditions
		validations = spec.Validations
		auditAnnotations = spec.AuditAnnotations
		variables = spec.Variables

		// convert celexceptions if exist
		for _, exception := range exceptions {
			if celpolex := exception.AsCELException(); celpolex != nil {
				for _, matchCondition := range celpolex.Spec.MatchConditions {
					// negate the match condition
					expression := "!(" + matchCondition.Expression + ")"
					if group != "" {
						// The exception was written against the Pod-shaped base object; rewrite it
						// the same way validations were rewritten for this autogen group, or an
						// expression like object.spec.containers... would silently never match (or
						// error) once object is actually a Deployment/CronJob.
						expression = string(autogen.Apply([]byte(expression), autogen.ReplacementsMap[group]...))
					}
					matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition{
						Name:       matchCondition.Name,
						Expression: expression,
					})
				}
				if len(celpolex.Spec.Images) > 0 {
					quotedImages := make([]string, len(celpolex.Spec.Images))
					for i, img := range celpolex.Spec.Images {
						quotedImages[i] = fmt.Sprintf("'%s'", img)
					}
					variables = append(variables, admissionregistrationv1.Variable{
						Name:       "allowedImages",
						Expression: fmt.Sprintf("[%s]", strings.Join(quotedImages, ", ")),
					})
				}
				if len(celpolex.Spec.AllowedValues) > 0 {
					quotedValues := make([]string, len(celpolex.Spec.AllowedValues))
					for i, val := range celpolex.Spec.AllowedValues {
						quotedValues[i] = fmt.Sprintf("'%s'", val)
					}
					variables = append(variables, admissionregistrationv1.Variable{
						Name:       "allowedValues",
						Expression: fmt.Sprintf("[%s]", strings.Join(quotedValues, ", ")),
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

// BuildValidatingAdmissionPolicyBinding is used to build a Kubernetes ValidatingAdmissionPolicyBinding
// from a Kyverno policy. vapName is the name of the ValidatingAdmissionPolicy this binding targets -
// callers already know it (they just created/looked up that object), so it's taken explicitly rather
// than re-derived here, which lets one policy bind to several generated VAPs (the base one plus one
// per autogen group). specOverride mirrors BuildValidatingAdmissionPolicy's parameter of the same name.
func BuildValidatingAdmissionPolicyBinding(
	vapbinding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	policy engineapi.GenericPolicy,
	vapName string,
	specOverride *policiesv1beta1.ValidatingPolicySpec,
) error {
	var validationActions []admissionregistrationv1.ValidationAction
	var paramRef *admissionregistrationv1.ParamRef

	if cpol := policy.AsKyvernoPolicy(); cpol != nil {
		rule := cpol.GetSpec().Rules[0]
		validateAction := rule.Validation.FailureAction
		if validateAction != nil {
			if validateAction.Enforce() {
				validationActions = append(validationActions, admissionregistrationv1.Deny)
			} else if validateAction.Audit() {
				validationActions = append(validationActions, admissionregistrationv1.Audit)
				validationActions = append(validationActions, admissionregistrationv1.Warn)
			}
		} else {
			validateAction := cpol.GetSpec().ValidationFailureAction
			if validateAction.Enforce() {
				validationActions = append(validationActions, admissionregistrationv1.Deny)
			} else if validateAction.Audit() {
				validationActions = append(validationActions, admissionregistrationv1.Audit)
				validationActions = append(validationActions, admissionregistrationv1.Warn)
			}
		}
		paramRef = rule.Validation.CEL.ParamRef
	} else if vpol := policy.AsValidatingPolicy(); vpol != nil {
		spec := vpol.Spec
		if specOverride != nil {
			spec = *specOverride
		}
		validationActions = spec.ValidationActions()
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
		PolicyName:        vapName,
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

// negateExceptionMatchConditions returns negated match conditions for each exception, using the v1
// MatchCondition type which is structurally identical to the alpha/beta variants. group is the
// matching mpol autogen config key ("" for the base/non-autogen'd variant) - see
// BuildValidatingAdmissionPolicy's group parameter for why this rewrite is needed: without it, an
// exception written against the Pod-shaped base object would silently misbehave once embedded into an
// autogen-group MutatingAdmissionPolicy whose object is a Deployment/CronJob.
func negateExceptionMatchConditions(exceptions []policiesv1beta1.PolicyException, group string) []admissionregistrationv1.MatchCondition {
	var result []admissionregistrationv1.MatchCondition
	for _, exception := range exceptions {
		for _, mc := range exception.Spec.MatchConditions {
			expression := "!(" + mc.Expression + ")"
			if group != "" {
				expression = mpolautogen.ConvertPodToTemplateExpression(expression, group)
			}
			result = append(result, admissionregistrationv1.MatchCondition{
				Name:       mc.Name,
				Expression: expression,
			})
		}
	}
	return result
}

// BuildMutatingAdmissionPolicy is used to build a Kubernetes MutatingAdmissionPolicy from a MutatingPolicy.
// specOverride, when non-nil, is used in place of mp's own spec - see BuildValidatingAdmissionPolicy's
// parameter of the same name for why (autogen fan-out). group is threaded through to
// negateExceptionMatchConditions for the same reason.
func BuildMutatingAdmissionPolicy(
	mapol *admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	mp *policiesv1beta1.MutatingPolicy,
	exceptions []policiesv1beta1.PolicyException,
	group string,
	specOverride *policiesv1beta1.MutatingPolicySpec,
) {
	spec := mp.Spec
	if specOverride != nil {
		spec = *specOverride
	}
	matchConditions := slicesutils.Map(negateExceptionMatchConditions(exceptions, group), func(mc admissionregistrationv1.MatchCondition) admissionregistrationv1alpha1.MatchCondition {
		return admissionregistrationv1alpha1.MatchCondition(mc)
	})
	for _, mc := range spec.MatchConditions {
		matchConditions = append(matchConditions, admissionregistrationv1alpha1.MatchCondition(mc))
	}

	var fpt *admissionregistrationv1alpha1.FailurePolicyType
	if spec.FailurePolicy != nil {
		conv := admissionregistrationv1alpha1.FailurePolicyType(*spec.FailurePolicy)
		fpt = &conv
	}

	// set owner reference
	mapol.OwnerReferences = mutatingPolicyOwnerRef(mp)
	// set policy spec
	mapol.Spec = admissionregistrationv1alpha1.MutatingAdmissionPolicySpec{
		MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
			ResourceRules: slicesutils.Map(spec.MatchConstraints.ResourceRules, func(rule admissionregistrationv1.NamedRuleWithOperations) admissionregistrationv1alpha1.NamedRuleWithOperations {
				return admissionregistrationv1alpha1.NamedRuleWithOperations{
					ResourceNames:      rule.ResourceNames,
					RuleWithOperations: rule.RuleWithOperations,
				}
			}),
		},
		MatchConditions: matchConditions,
		Mutations:       spec.Mutations,
		Variables: slicesutils.Map(spec.Variables, func(v admissionregistrationv1.Variable) admissionregistrationv1alpha1.Variable {
			return admissionregistrationv1alpha1.Variable(v)
		}),
		FailurePolicy:      fpt,
		ReinvocationPolicy: spec.GetReinvocationPolicy(),
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(mapol)
	policyLabels := mp.GetLabels()
	if _, ok := policyLabels[kyverno.LabelExcludeReporting]; ok {
		mapol.Labels[kyverno.LabelExcludeReporting] = "true"
	}
}

// BuildMutatingAdmissionPolicyBinding is used to build a Kubernetes MutatingAdmissionPolicyBinding from
// a MutatingPolicy. mapName is the name of the MutatingAdmissionPolicy this binding targets - see
// BuildValidatingAdmissionPolicyBinding's vapName parameter for why it's taken explicitly.
func BuildMutatingAdmissionPolicyBinding(
	mapbinding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	mp *policiesv1beta1.MutatingPolicy,
	mapName string,
) {
	mapbinding.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapbinding.Spec = admissionregistrationv1alpha1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: mapName,
	}
	controllerutils.SetManagedByKyvernoLabel(mapbinding)
}

// BuildMutatingAdmissionPolicyV1 is used to build a Kubernetes MutatingAdmissionPolicy (v1) from a
// MutatingPolicy. group and specOverride mirror BuildMutatingAdmissionPolicy's parameters of the same name.
func BuildMutatingAdmissionPolicyV1(
	mapol *admissionregistrationv1.MutatingAdmissionPolicy,
	mp *policiesv1beta1.MutatingPolicy,
	exceptions []policiesv1beta1.PolicyException,
	group string,
	specOverride *policiesv1beta1.MutatingPolicySpec,
) {
	spec := mp.Spec
	if specOverride != nil {
		spec = *specOverride
	}
	matchConditions := negateExceptionMatchConditions(exceptions, group)
	for _, mc := range spec.MatchConditions {
		matchConditions = append(matchConditions, mc)
	}

	var fpt *admissionregistrationv1.FailurePolicyType
	if spec.FailurePolicy != nil {
		conv := *spec.FailurePolicy
		fpt = &conv
	}

	mapol.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapol.Spec = admissionregistrationv1.MutatingAdmissionPolicySpec{
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: slicesutils.Map(spec.MatchConstraints.ResourceRules, func(rule admissionregistrationv1.NamedRuleWithOperations) admissionregistrationv1.NamedRuleWithOperations {
				return admissionregistrationv1.NamedRuleWithOperations{
					ResourceNames:      rule.ResourceNames,
					RuleWithOperations: rule.RuleWithOperations,
				}
			}),
		},
		MatchConditions: matchConditions,
		Mutations: slicesutils.Map(spec.Mutations, func(m admissionregistrationv1alpha1.Mutation) admissionregistrationv1.Mutation {
			mut := admissionregistrationv1.Mutation{PatchType: admissionregistrationv1.PatchType(m.PatchType)}
			if m.ApplyConfiguration != nil {
				mut.ApplyConfiguration = &admissionregistrationv1.ApplyConfiguration{Expression: m.ApplyConfiguration.Expression}
			}
			if m.JSONPatch != nil {
				mut.JSONPatch = &admissionregistrationv1.JSONPatch{Expression: m.JSONPatch.Expression}
			}
			return mut
		}),
		Variables: slicesutils.Map(spec.Variables, func(v admissionregistrationv1.Variable) admissionregistrationv1.Variable {
			return v
		}),
		FailurePolicy:      fpt,
		ReinvocationPolicy: spec.GetReinvocationPolicy(),
	}
	controllerutils.SetManagedByKyvernoLabel(mapol)
	policyLabels := mp.GetLabels()
	if _, ok := policyLabels[kyverno.LabelExcludeReporting]; ok {
		mapol.Labels[kyverno.LabelExcludeReporting] = "true"
	}
}

// BuildMutatingAdmissionPolicyBindingV1 is used to build a Kubernetes MutatingAdmissionPolicyBinding
// (v1) from a MutatingPolicy. mapName is the name of the MutatingAdmissionPolicy this binding targets -
// see BuildValidatingAdmissionPolicyBinding's vapName parameter for why it's taken explicitly.
func BuildMutatingAdmissionPolicyBindingV1(
	mapbinding *admissionregistrationv1.MutatingAdmissionPolicyBinding,
	mp *policiesv1beta1.MutatingPolicy,
	mapName string,
) {
	mapbinding.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapbinding.Spec = admissionregistrationv1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: mapName,
	}
	controllerutils.SetManagedByKyvernoLabel(mapbinding)
}

// BuildMutatingAdmissionPolicyBeta is used to build a Kubernetes MutatingAdmissionPolicy (v1beta1) from
// a MutatingPolicy. group and specOverride mirror BuildMutatingAdmissionPolicy's parameters of the same name.
func BuildMutatingAdmissionPolicyBeta(
	mapol *admissionregistrationv1beta1.MutatingAdmissionPolicy,
	mp *policiesv1beta1.MutatingPolicy,
	exceptions []policiesv1beta1.PolicyException,
	group string,
	specOverride *policiesv1beta1.MutatingPolicySpec,
) {
	spec := mp.Spec
	if specOverride != nil {
		spec = *specOverride
	}
	matchConditions := slicesutils.Map(negateExceptionMatchConditions(exceptions, group), func(mc admissionregistrationv1.MatchCondition) admissionregistrationv1beta1.MatchCondition {
		return admissionregistrationv1beta1.MatchCondition(mc)
	})
	for _, mc := range spec.MatchConditions {
		matchConditions = append(matchConditions, admissionregistrationv1beta1.MatchCondition(mc))
	}

	var fpt *admissionregistrationv1beta1.FailurePolicyType
	if spec.FailurePolicy != nil {
		conv := admissionregistrationv1beta1.FailurePolicyType(*spec.FailurePolicy)
		fpt = &conv
	}

	// set owner reference
	mapol.OwnerReferences = mutatingPolicyOwnerRef(mp)
	// set policy spec
	mapol.Spec = admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
		MatchConstraints: &admissionregistrationv1beta1.MatchResources{
			ResourceRules: slicesutils.Map(spec.MatchConstraints.ResourceRules, func(rule admissionregistrationv1.NamedRuleWithOperations) admissionregistrationv1beta1.NamedRuleWithOperations {
				return admissionregistrationv1beta1.NamedRuleWithOperations{
					ResourceNames:      rule.ResourceNames,
					RuleWithOperations: rule.RuleWithOperations,
				}
			}),
		},
		MatchConditions: matchConditions,
		Mutations: slicesutils.Map(spec.Mutations, func(m admissionregistrationv1alpha1.Mutation) admissionregistrationv1beta1.Mutation {
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
		Variables: slicesutils.Map(spec.Variables, func(v admissionregistrationv1.Variable) admissionregistrationv1beta1.Variable {
			return admissionregistrationv1beta1.Variable(v)
		}),
		FailurePolicy:      fpt,
		ReinvocationPolicy: spec.GetReinvocationPolicy(),
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(mapol)
	policyLabels := mp.GetLabels()
	if _, ok := policyLabels[kyverno.LabelExcludeReporting]; ok {
		mapol.Labels[kyverno.LabelExcludeReporting] = "true"
	}
}

// BuildMutatingAdmissionPolicyBindingBeta is used to build a Kubernetes MutatingAdmissionPolicyBinding
// (v1beta1) from a MutatingPolicy. mapName is the name of the MutatingAdmissionPolicy this binding
// targets - see BuildValidatingAdmissionPolicyBinding's vapName parameter for why it's taken explicitly.
func BuildMutatingAdmissionPolicyBindingBeta(
	mapbinding *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding,
	mp *policiesv1beta1.MutatingPolicy,
	mapName string,
) {
	mapbinding.OwnerReferences = mutatingPolicyOwnerRef(mp)
	mapbinding.Spec = admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: mapName,
	}
	controllerutils.SetManagedByKyvernoLabel(mapbinding)
}

func translateResourceFilters(discoveryClient dclient.IDiscovery,
	matchResources *admissionregistrationv1.MatchResources,
	rules *[]admissionregistrationv1.NamedRuleWithOperations,
	resFilters kyvernov1.ResourceFilters,
	isMatch bool,
) error {
	for _, filter := range resFilters {
		err := translateResource(discoveryClient, matchResources, rules, filter.ResourceDescription, isMatch)
		if err != nil {
			return err
		}
	}
	return nil
}

func translateResource(
	discoveryClient dclient.IDiscovery,
	matchResources *admissionregistrationv1.MatchResources,
	rules *[]admissionregistrationv1.NamedRuleWithOperations,
	res kyvernov1.ResourceDescription,
	isMatch bool,
) error {
	err := constructValidatingAdmissionPolicyRules(discoveryClient, rules, res, isMatch)
	if err != nil {
		return err
	}

	if isMatch {
		matchResources.ResourceRules = *rules
		if len(res.Namespaces) > 0 {
			namespaceSelector := &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: "In",
						Values:   res.Namespaces,
					},
				},
			}
			matchResources.NamespaceSelector = namespaceSelector
		} else {
			matchResources.NamespaceSelector = res.NamespaceSelector
		}
		matchResources.ObjectSelector = res.Selector
	} else {
		matchResources.ExcludeResourceRules = *rules
	}
	return nil
}

func constructValidatingAdmissionPolicyRules(
	discoveryClient dclient.IDiscovery,
	rules *[]admissionregistrationv1.NamedRuleWithOperations,
	res kyvernov1.ResourceDescription,
	isMatch bool,
) error {
	// translate operations to their corresponding values in validating admission policy.
	ops := translateOperations(res.GetOperations())

	resourceNames := res.Names
	if res.Name != "" {
		resourceNames = append(resourceNames, res.Name)
	}

	// get kinds from kyverno policies and translate them to rules in validating admission policies.
	// matched resources in kyverno policies are written in the following format:
	// group/version/kind/subresource
	// whereas matched resources in validating admission policies are written in the following format:
	// apiGroups:   ["group"]
	// apiVersions: ["version"]
	// resources:   ["resource"]
	for _, kind := range res.Kinds {
		var r admissionregistrationv1.NamedRuleWithOperations

		if kind == "*" {
			r = buildNamedRuleWithOperations(resourceNames, "*", "*", ops, "*")
			*rules = append(*rules, r)
		} else {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrss, err := discoveryClient.FindResources(group, version, kind, subresource)
			if err != nil {
				return err
			}
			if len(gvrss) != 1 {
				return fmt.Errorf("no unique match for kind %s", kind)
			}

			for topLevelApi, apiResource := range gvrss {
				resources := []string{apiResource.Name}

				// Add pods/ephemeralcontainers if pods resource.
				if apiResource.Name == "pods" {
					resources = append(resources, "pods/ephemeralcontainers")
				}

				// Check if there's an existing rule for the same group and version.
				var isNewRule bool = true
				for i := range *rules {
					if slices.Contains((*rules)[i].APIGroups, topLevelApi.Group) && slices.Contains((*rules)[i].APIVersions, topLevelApi.Version) {
						(*rules)[i].Resources = append((*rules)[i].Resources, resources...)
						slices.Sort((*rules)[i].Resources)
						isNewRule = false
						break
					}
				}

				// If no existing rule found, create a new one.
				if isNewRule {
					r = buildNamedRuleWithOperations(resourceNames, topLevelApi.Group, topLevelApi.Version, ops, resources...)
					*rules = append(*rules, r)
				}
			}
		}
	}

	// if exclude block has namespaces but no kinds, we need to add a rule for the namespaces
	if !isMatch && len(res.Namespaces) > 0 && len(res.Kinds) == 0 {
		r := admissionregistrationv1.NamedRuleWithOperations{
			ResourceNames: res.Namespaces,
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					Resources:   []string{"namespaces"},
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
				},
				Operations: ops,
			},
		}
		*rules = append(*rules, r)
	}
	return nil
}

func buildNamedRuleWithOperations(
	resourceNames []string,
	group, version string,
	operations []admissionregistrationv1.OperationType,
	resources ...string,
) admissionregistrationv1.NamedRuleWithOperations {
	slices.Sort(resources)
	return admissionregistrationv1.NamedRuleWithOperations{
		ResourceNames: resourceNames,
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Rule: admissionregistrationv1.Rule{
				Resources:   resources,
				APIGroups:   []string{group},
				APIVersions: []string{version},
			},
			Operations: operations,
		},
	}
}

func translateOperations(operations []string) []admissionregistrationv1.OperationType {
	var vapOperations []admissionregistrationv1.OperationType
	for _, op := range operations {
		if op == string(kyvernov1.Create) {
			vapOperations = append(vapOperations, admissionregistrationv1.Create)
		} else if op == string(kyvernov1.Update) {
			vapOperations = append(vapOperations, admissionregistrationv1.Update)
		} else if op == string(kyvernov1.Connect) {
			vapOperations = append(vapOperations, admissionregistrationv1.Connect)
		} else if op == string(kyvernov1.Delete) {
			vapOperations = append(vapOperations, admissionregistrationv1.Delete)
		}
	}

	// set default values for operations since it's a required field in ValidatingAdmissionPolicies
	if len(vapOperations) == 0 {
		vapOperations = append(vapOperations, admissionregistrationv1.Create)
		vapOperations = append(vapOperations, admissionregistrationv1.Update)
		vapOperations = append(vapOperations, admissionregistrationv1.Connect)
		vapOperations = append(vapOperations, admissionregistrationv1.Delete)
	}
	return vapOperations
}
