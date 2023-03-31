package common

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type KyvernoMatchedResources struct {
	policies []kyvernov1.PolicyInterface
}

func (r *KyvernoMatchedResources) GetMatchedResources(resourcePaths []string, dClient dclient.Interface, namespace string, policyReport bool) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var err error

	resourceTypesMap := make(map[schema.GroupVersionKind]bool)
	var resourceTypes []schema.GroupVersionKind
	var subresourceMap map[schema.GroupVersionKind]Subresource

	for _, policy := range r.policies {
		for _, rule := range autogen.ComputeRules(policy) {
			var resourceTypesInRule map[schema.GroupVersionKind]bool
			resourceTypesInRule, subresourceMap = GetKindsFromRule(rule, dClient)
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
