package internal

import (
	"github.com/go-logr/logr"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func MatchPolicyContext(logger logr.Logger, policyContext engineapi.PolicyContext) bool {
	match := matchPolicyContext(policyContext)
	if !match {
		logger.V(2).Info("policy context does not match")
	}
	return match
}

func matchPolicyContext(policyContext engineapi.PolicyContext) bool {
	policy := policyContext.Policy()
	if policy.IsNamespaced() {
		policyNamespace := policy.GetNamespace()
		if resource := policyContext.NewResource(); resource.Object != nil {
			resourceNamespace := resource.GetNamespace()
			if resourceNamespace != policyNamespace || resourceNamespace == "" {
				return false
			}
		}
		if resource := policyContext.OldResource(); resource.Object != nil {
			resourceNamespace := resource.GetNamespace()
			if resourceNamespace != policyNamespace || resourceNamespace == "" {
				return false
			}
		}
	}
	return true
}
