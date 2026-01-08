package policy

import (
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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

	if !isNamespacedPolicy && len(match.Namespaces) > 0 && !containsIncludingWildcards(match.Namespaces, res.GetNamespace()) {
		return false
	}
	return true
}

func containsIncludingWildcards(slice []string, item string) bool {
	for _, s := range slice {
		if wildcard.Match(s, item) {
			return true
		}
	}
	return false
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
	case *policiesv1beta1.GeneratingPolicy:
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

func ParsePolicyKey(policy string) (string, string) {
	parts := strings.Split(policy, "/")
	if len(parts) == 2 {
		return parts[1], parts[0]
	}
	return parts[0], ""
}
