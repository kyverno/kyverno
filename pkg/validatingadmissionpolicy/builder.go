package validatingadmissionpolicy

import (
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildValidatingAdmissionPolicy is used to build a Kubernetes ValidatingAdmissionPolicy from a Kyverno policy
func BuildValidatingAdmissionPolicy(
	discoveryClient dclient.IDiscovery,
	vap *admissionregistrationv1beta1.ValidatingAdmissionPolicy,
	cpol kyvernov1.PolicyInterface,
	exceptions []kyvernov2.PolicyException,
) error {
	// set owner reference
	vap.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "kyverno.io/v1",
			Kind:       cpol.GetKind(),
			Name:       cpol.GetName(),
			UID:        cpol.GetUID(),
		},
	}

	// construct the rules
	var matchResources admissionregistrationv1beta1.MatchResources
	var matchRules, excludeRules []admissionregistrationv1beta1.NamedRuleWithOperations

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
		match := exception.Spec.Match
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

	// set policy spec
	vap.Spec = admissionregistrationv1beta1.ValidatingAdmissionPolicySpec{
		MatchConstraints: &matchResources,
		ParamKind:        rule.Validation.CEL.ParamKind,
		Variables:        rule.Validation.CEL.Variables,
		Validations:      rule.Validation.CEL.Expressions,
		AuditAnnotations: rule.Validation.CEL.AuditAnnotations,
		MatchConditions:  rule.CELPreconditions,
	}

	// set labels
	controllerutils.SetManagedByKyvernoLabel(vap)
	return nil
}

// BuildValidatingAdmissionPolicyBinding is used to build a Kubernetes ValidatingAdmissionPolicyBinding from a Kyverno policy
func BuildValidatingAdmissionPolicyBinding(
	vapbinding *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding,
	cpol kyvernov1.PolicyInterface,
) error {
	// set owner reference
	vapbinding.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "kyverno.io/v1",
			Kind:       cpol.GetKind(),
			Name:       cpol.GetName(),
			UID:        cpol.GetUID(),
		},
	}

	// set validation action for vap binding
	var validationActions []admissionregistrationv1beta1.ValidationAction
	validateAction := cpol.GetSpec().Rules[0].Validation.FailureAction
	if validateAction != nil {
		if validateAction.Enforce() {
			validationActions = append(validationActions, admissionregistrationv1beta1.Deny)
		} else if validateAction.Audit() {
			validationActions = append(validationActions, admissionregistrationv1beta1.Audit)
			validationActions = append(validationActions, admissionregistrationv1beta1.Warn)
		}
	} else {
		validateAction := cpol.GetSpec().ValidationFailureAction
		if validateAction.Enforce() {
			validationActions = append(validationActions, admissionregistrationv1beta1.Deny)
		} else if validateAction.Audit() {
			validationActions = append(validationActions, admissionregistrationv1beta1.Audit)
			validationActions = append(validationActions, admissionregistrationv1beta1.Warn)
		}
	}

	// set validating admission policy binding spec
	rule := cpol.GetSpec().Rules[0]
	vapbinding.Spec = admissionregistrationv1beta1.ValidatingAdmissionPolicyBindingSpec{
		PolicyName:        cpol.GetName(),
		ParamRef:          rule.Validation.CEL.ParamRef,
		ValidationActions: validationActions,
	}

	// set labels
	controllerutils.SetManagedByKyvernoLabel(vapbinding)
	return nil
}

func translateResourceFilters(discoveryClient dclient.IDiscovery,
	matchResources *admissionregistrationv1beta1.MatchResources,
	rules *[]admissionregistrationv1beta1.NamedRuleWithOperations,
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
	matchResources *admissionregistrationv1beta1.MatchResources,
	rules *[]admissionregistrationv1beta1.NamedRuleWithOperations,
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
	rules *[]admissionregistrationv1beta1.NamedRuleWithOperations,
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
		var r admissionregistrationv1beta1.NamedRuleWithOperations

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
		r := admissionregistrationv1beta1.NamedRuleWithOperations{
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
) admissionregistrationv1beta1.NamedRuleWithOperations {
	return admissionregistrationv1beta1.NamedRuleWithOperations{
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
