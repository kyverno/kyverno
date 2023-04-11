package common

import (
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ValidatingAdmissionResources struct {
	policies []v1alpha1.ValidatingAdmissionPolicy
}

func (r *ValidatingAdmissionResources) FetchResourcesFromPolicy(resourcePaths []string, dClient dclient.Interface, namespace string, policyReport bool) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var err error

	resourceTypesMap := make(map[schema.GroupVersionKind]bool)
	var resourceTypes []schema.GroupVersionKind
	var subresourceMap map[schema.GroupVersionKind]Subresource

	for _, policy := range r.policies {
		for _, rule := range policy.Spec.MatchConstraints.ResourceRules {
			var resourceTypesInRule map[schema.GroupVersionKind]bool
			resourceTypesInRule, subresourceMap = getKindsFromValidatingAdmissionRule(rule.RuleWithOperations.Rule, dClient)
			for resourceKind := range resourceTypesInRule {
				resourceTypesMap[resourceKind] = true
			}
		}
	}

	for kind := range resourceTypesMap {
		resourceTypes = append(resourceTypes, kind)
	}

	resources, err = whenClusterIsTrue(resourceTypes, subresourceMap, dClient, namespace, resourcePaths, policyReport)
	return resources, err
}
