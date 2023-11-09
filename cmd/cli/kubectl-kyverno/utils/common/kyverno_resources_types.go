package common

import (
	"io"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type KyvernoResources struct {
	policies []kyvernov1.PolicyInterface
}

func (r *KyvernoResources) FetchResourcesFromPolicy(out io.Writer, resourcePaths []string, dClient dclient.Interface, namespace string, policyReport bool) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	var err error

	resourceTypesMap := make(map[schema.GroupVersionKind]bool)
	var resourceTypes []schema.GroupVersionKind
	var subresourceMap map[schema.GroupVersionKind]v1alpha1.Subresource

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

	resources, err = whenClusterIsTrue(out, resourceTypes, subresourceMap, dClient, namespace, resourcePaths, policyReport)

	return resources, err
}
