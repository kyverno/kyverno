package internal

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func MatchPolicyContext(logger logr.Logger, policyContext engineapi.PolicyContext, configuration config.Configuration) bool {
	policy := policyContext.Policy()
	old := policyContext.OldResource()
	new := policyContext.NewResource()
	if !checkNamespacedPolicy(policy, new, old) {
		logger.V(2).Info("policy namespace doesn't match resource namespace")
		return false
	}
	gvk, subresource := policyContext.ResourceKind()
	if !checkResourceFilters(configuration, gvk, subresource, new, old) {
		logger.V(2).Info("configuration resource filters doesn't match resource")
		return false
	}
	return true
}

func checkResourceFilters(configuration config.Configuration, gvk schema.GroupVersionKind, subresource string, resources ...unstructured.Unstructured) bool {
	for _, resource := range resources {
		if resource.Object != nil {
			// TODO: account for generate name here ?
			if configuration.ToFilter(gvk, subresource, resource.GetNamespace(), resource.GetName()) {
				return false
			}
		}
	}
	return true
}

func checkNamespacedPolicy(policy kyvernov1.PolicyInterface, resources ...unstructured.Unstructured) bool {
	if policy.IsNamespaced() {
		policyNamespace := policy.GetNamespace()
		for _, resource := range resources {
			if resource.Object != nil {
				resourceNamespace := resource.GetNamespace()
				if resourceNamespace != policyNamespace || resourceNamespace == "" {
					return false
				}
			}
		}
	}
	return true
}
