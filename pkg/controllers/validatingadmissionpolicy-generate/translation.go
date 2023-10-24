package validatingadmissionpolicygenerate

import (
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
)

func (c *controller) translateResourceFilters(matchResources *v1alpha1.MatchResources, rules *[]v1alpha1.NamedRuleWithOperations, resFilters kyvernov1.ResourceFilters) error {
	for _, filter := range resFilters {
		err := c.translateResource(matchResources, rules, filter.ResourceDescription)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) translateResource(matchResources *v1alpha1.MatchResources, rules *[]v1alpha1.NamedRuleWithOperations, res kyvernov1.ResourceDescription) error {
	err := c.constructValidatingAdmissionPolicyRules(rules, res.Kinds, res.GetOperations())
	if err != nil {
		return err
	}

	matchResources.ResourceRules = *rules
	matchResources.NamespaceSelector = res.NamespaceSelector
	matchResources.ObjectSelector = res.Selector
	return nil
}

func (c *controller) constructValidatingAdmissionPolicyRules(rules *[]v1alpha1.NamedRuleWithOperations, kinds []string, operations []string) error {
	// translate operations to their corresponding values in validating admission policy.
	ops := c.translateOperations(operations)

	// get kinds from kyverno policies and translate them to rules in validating admission policies.
	// matched resources in kyverno policies are written in the following format:
	// group/version/kind/subresource
	// whereas matched resources in validating admission policies are written in the following format:
	// apiGroups:   ["group"]
	// apiVersions: ["version"]
	// resources:   ["resource"]
	for _, kind := range kinds {
		group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
		gvrss, err := c.discoveryClient.FindResources(group, version, kind, subresource)
		if err != nil {
			return err
		}
		if len(gvrss) != 1 {
			return fmt.Errorf("no unique match for kind %s", kind)
		}

		for topLevelApi, apiResource := range gvrss {
			isNewRule := true
			// If there's a rule that contains both group and version, then the resource is appended to the existing rule instead of creating a new one.
			// Example:  apiGroups:   ["apps"]
			//           apiVersions: ["v1"]
			//           resources:   ["deployments", "statefulsets"]
			// Otherwise, a new rule is created.
			for i := range *rules {
				if slices.Contains((*rules)[i].APIGroups, topLevelApi.Group) && slices.Contains((*rules)[i].APIVersions, topLevelApi.Version) {
					(*rules)[i].Resources = append((*rules)[i].Resources, apiResource.Name)
					isNewRule = false
					break
				}
			}
			if isNewRule {
				r := v1alpha1.NamedRuleWithOperations{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							Resources:   []string{apiResource.Name},
							APIGroups:   []string{topLevelApi.Group},
							APIVersions: []string{topLevelApi.Version},
						},
						Operations: ops,
					},
				}
				*rules = append(*rules, r)
			}
		}
	}
	return nil
}

func (c *controller) translateOperations(operations []string) []admissionregistrationv1.OperationType {
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
