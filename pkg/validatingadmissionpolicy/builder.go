package validatingadmissionpolicy

import (
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildValidatingAdmissionPolicy is used to build a Kubernetes ValidatingAdmissionPolicy from a Kyverno policy
func BuildValidatingAdmissionPolicy(discoveryClient dclient.IDiscovery, vap *admissionregistrationv1alpha1.ValidatingAdmissionPolicy, cpol kyvernov1.PolicyInterface) error {
	// set owner reference
	vap.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "kyverno.io/v1",
			Kind:       cpol.GetKind(),
			Name:       cpol.GetName(),
			UID:        cpol.GetUID(),
		},
	}

	// construct validating admission policy resource rules
	var matchResources admissionregistrationv1alpha1.MatchResources
	var matchRules []admissionregistrationv1alpha1.NamedRuleWithOperations

	rule := cpol.GetSpec().Rules[0]
	match := rule.MatchResources
	if !match.ResourceDescription.IsEmpty() {
		if err := translateResource(discoveryClient, &matchResources, &matchRules, match.ResourceDescription); err != nil {
			return err
		}
	}

	if match.Any != nil {
		if err := translateResourceFilters(discoveryClient, &matchResources, &matchRules, match.Any); err != nil {
			return err
		}
	}
	if match.All != nil {
		if err := translateResourceFilters(discoveryClient, &matchResources, &matchRules, match.All); err != nil {
			return err
		}
	}

	// set validating admission policy spec
	vap.Spec = admissionregistrationv1alpha1.ValidatingAdmissionPolicySpec{
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
func BuildValidatingAdmissionPolicyBinding(vapbinding *admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding, cpol kyvernov1.PolicyInterface) error {
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
	var validationActions []admissionregistrationv1alpha1.ValidationAction
	action := cpol.GetSpec().ValidationFailureAction
	if action.Enforce() {
		validationActions = append(validationActions, admissionregistrationv1alpha1.Deny)
	} else if action.Audit() {
		validationActions = append(validationActions, admissionregistrationv1alpha1.Audit)
		validationActions = append(validationActions, admissionregistrationv1alpha1.Warn)
	}

	// set validating admission policy binding spec
	rule := cpol.GetSpec().Rules[0]
	vapbinding.Spec = admissionregistrationv1alpha1.ValidatingAdmissionPolicyBindingSpec{
		PolicyName:        cpol.GetName(),
		ParamRef:          rule.Validation.CEL.ParamRef,
		ValidationActions: validationActions,
	}

	// set labels
	controllerutils.SetManagedByKyvernoLabel(vapbinding)
	return nil
}

func translateResourceFilters(discoveryClient dclient.IDiscovery, matchResources *admissionregistrationv1alpha1.MatchResources, rules *[]admissionregistrationv1alpha1.NamedRuleWithOperations, resFilters kyvernov1.ResourceFilters) error {
	for _, filter := range resFilters {
		err := translateResource(discoveryClient, matchResources, rules, filter.ResourceDescription)
		if err != nil {
			return err
		}
	}
	return nil
}

func translateResource(discoveryClient dclient.IDiscovery, matchResources *admissionregistrationv1alpha1.MatchResources, rules *[]admissionregistrationv1alpha1.NamedRuleWithOperations, res kyvernov1.ResourceDescription) error {
	err := constructValidatingAdmissionPolicyRules(discoveryClient, rules, res)
	if err != nil {
		return err
	}

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
	return nil
}

func constructValidatingAdmissionPolicyRules(discoveryClient dclient.IDiscovery, rules *[]admissionregistrationv1alpha1.NamedRuleWithOperations, res kyvernov1.ResourceDescription) error {
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
		var r admissionregistrationv1alpha1.NamedRuleWithOperations

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
	return nil
}

func buildNamedRuleWithOperations(
	resourceNames []string,
	group, version string,
	operations []admissionregistrationv1.OperationType,
	resources ...string,
) admissionregistrationv1alpha1.NamedRuleWithOperations {
	return admissionregistrationv1alpha1.NamedRuleWithOperations{
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

	// set default values for operations since it's a required field in validating admission policies
	if len(vapOperations) == 0 {
		vapOperations = append(vapOperations, admissionregistrationv1.Create)
		vapOperations = append(vapOperations, admissionregistrationv1.Update)
	}
	return vapOperations
}
