package common

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test/api"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ValidatingAdmissionResources struct {
	policies []v1alpha1.ValidatingAdmissionPolicy
}

func (r *ValidatingAdmissionResources) FetchResourcesFromPolicy(resourcePaths []string, dClient dclient.Interface, namespace string, policyReport bool) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	var err error

	resourceTypesMap := make(map[schema.GroupVersionKind]bool)
	var resourceTypes []schema.GroupVersionKind
	var subresourceMap map[schema.GroupVersionKind]api.Subresource

	for _, policy := range r.policies {
		var resourceTypesInRule map[schema.GroupVersionKind]bool
		resourceTypesInRule, subresourceMap = getKindsFromValidatingAdmissionPolicy(policy, dClient)
		if err != nil {
			return resources, err
		}
		for resourceKind := range resourceTypesInRule {
			resourceTypesMap[resourceKind] = true
		}
	}

	for kind := range resourceTypesMap {
		resourceTypes = append(resourceTypes, kind)
	}

	resources, err = whenClusterIsTrue(resourceTypes, subresourceMap, dClient, namespace, resourcePaths, policyReport)
	return resources, err
}
