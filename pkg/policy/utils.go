package policy

import (
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// castPolicy only recognizes CEL policy kinds (gpol/ngpol/mpol/nmpol); any
// other input, including legacy kyverno.io types, returns nil.
func castPolicy(p interface{}) engineapi.GenericPolicy {
	var policy engineapi.GenericPolicy
	switch obj := p.(type) {
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

func ParsePolicyKey(policy string) (string, string) {
	parts := strings.Split(policy, "/")
	if len(parts) == 2 {
		return parts[1], parts[0]
	}
	return parts[0], ""
}
