package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func resourceMatches(match kyvernov1.ResourceDescription, res unstructured.Unstructured, isNamespacedPolicy bool) bool {
	if match.Name != "" && res.GetName() != match.Name {
		return false
	}
	if len(match.Names) > 0 && !contains(match.Names, res.GetName()) {
		return false
	}
	if !isNamespacedPolicy && len(match.Namespaces) > 0 && !contains(match.Namespaces, res.GetNamespace()) {
		return false
	}
	return true
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func castPolicy(p interface{}) kyvernov1.PolicyInterface {
	var policy kyvernov1.PolicyInterface
	switch obj := p.(type) {
	case *kyvernov1.ClusterPolicy:
		policy = obj
	case *kyvernov1.Policy:
		policy = obj
	}
	return policy
}
