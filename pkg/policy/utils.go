package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/ext/wildcard"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func resourceMatches(match kyvernov1.ResourceDescription, res unstructured.Unstructured, isNamespacedPolicy bool) bool {
	if match.Name != "" && !wildcard.Match(match.Name, res.GetName()) {
		return false
	}

	if len(match.Names) > 0 {
		isMatch := false
		for _, name := range match.Names {
			if wildcard.Match(name, res.GetName()) {
				isMatch = true
				break
			}
		}
		if !isMatch {
			return false
		}
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

func castPolicy(p interface{}) engineapi.GenericPolicy {
	var policy engineapi.GenericPolicy
	switch obj := p.(type) {
	case *kyvernov1.ClusterPolicy:
		policy = engineapi.NewKyvernoPolicy(obj)
	case *kyvernov1.Policy:
		policy = engineapi.NewKyvernoPolicy(obj)
	case *policiesv1alpha1.GeneratingPolicy:
		policy = engineapi.NewGeneratingPolicy(obj)
	}
	return policy
}

func policyKey(policy kyvernov1.PolicyInterface) string {
	var policyNameNamespaceKey string

	if policy.IsNamespaced() {
		policyNameNamespaceKey = policy.GetNamespace() + "/" + policy.GetName()
	} else {
		policyNameNamespaceKey = policy.GetName()
	}
	return policyNameNamespaceKey
}
