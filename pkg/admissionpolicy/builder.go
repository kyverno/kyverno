package admissionpolicy

import (
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
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
			}
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
		policyName = "cpol-" + cpol.GetName()
	} else if vpol := policy.AsValidatingPolicy(); vpol != nil {
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

// BuildMutatingAdmissionPolicy is used to build a Kubernetes MutatingAdmissionPolicy from a MutatingPolicy
func BuildMutatingAdmissionPolicy(
	mapol *admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	mp *policiesv1alpha1.MutatingPolicy,
	exceptions []policiesv1alpha1.PolicyException,
) {
	var matchConditions []admissionregistrationv1alpha1.MatchCondition
	// convert celexceptions if exist
	for _, exception := range exceptions {
		for _, matchCondition := range exception.Spec.MatchConditions {
			// negate the match condition
			expression := "!(" + matchCondition.Expression + ")"
			matchConditions = append(matchConditions, admissionregistrationv1alpha1.MatchCondition{
				Name:       matchCondition.Name,
				Expression: expression,
			})
		}
	}
	matchConditions = append(matchConditions, mp.Spec.MatchConditions...)
	// set owner reference
	mapol.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: policiesv1alpha1.GroupVersion.String(),
			Kind:       mp.GetKind(),
			Name:       mp.GetName(),
			UID:        mp.GetUID(),
		},
	}
	// set policy spec
	mapol.Spec = admissionregistrationv1alpha1.MutatingAdmissionPolicySpec{
		MatchConstraints:   mp.Spec.MatchConstraints,
		MatchConditions:    matchConditions,
		Mutations:          mp.Spec.Mutations,
		Variables:          mp.Spec.Variables,
		FailurePolicy:      mp.Spec.FailurePolicy,
		ReinvocationPolicy: mp.Spec.GetReinvocationPolicy(),
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(mapol)
}

// BuildMutatingAdmissionPolicyBinding is used to build a Kubernetes MutatingAdmissionPolicyBinding from a MutatingPolicy
func BuildMutatingAdmissionPolicyBinding(
	mapbinding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	mp *policiesv1alpha1.MutatingPolicy,
) {
	// set owner reference
	mapbinding.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: policiesv1alpha1.GroupVersion.String(),
			Kind:       mp.GetKind(),
			Name:       mp.GetName(),
			UID:        mp.GetUID(),
		},
	}
	// set binding spec
	mapbinding.Spec = admissionregistrationv1alpha1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: "mpol-" + mp.GetName(),
	}
	// set labels
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
