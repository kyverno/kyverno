package policy

import (
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

	if !selectorMatches(match.Selector, res.GetLabels()) {
		return false
	}
	return true
}

// selectorMatches checks a resource's labels against a selector, resolving wildcard characters
// in matchLabels keys/values against the resource's own labels. Kubernetes label selectors don't
// support wildcards, so this is needed for resources that were listed without the selector applied
// server-side (see getResources).
func selectorMatches(selector *metav1.LabelSelector, resourceLabels map[string]string) bool {
	if selector == nil {
		return true
	}
	for k, v := range selector.MatchLabels {
		if !wildcard.ContainsWildcard(k) && !wildcard.ContainsWildcard(v) {
			if val, ok := resourceLabels[k]; !ok || val != v {
				return false
			}
			continue
		}
		matched := false
		for rk, rv := range resourceLabels {
			if wildcard.Match(k, rk) && wildcard.Match(v, rv) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(selector.MatchExpressions) > 0 {
		exprSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchExpressions: selector.MatchExpressions})
		if err != nil || !exprSelector.Matches(labels.Set(resourceLabels)) {
			return false
		}
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
	case *policiesv1beta1.NamespacedGeneratingPolicy:
		policy = engineapi.NewNamespacedGeneratingPolicy(obj)
	case *policiesv1beta1.MutatingPolicy:
		policy = engineapi.NewMutatingPolicy(obj)
	case *policiesv1beta1.NamespacedMutatingPolicy:
		policy = engineapi.NewNamespacedMutatingPolicy(obj)
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
