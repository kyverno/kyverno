package utils

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/policy"
	"k8s.io/apimachinery/pkg/util/sets"
)

func CanBackgroundProcess(logger logr.Logger, p kyvernov1.PolicyInterface) bool {
	if !p.BackgroundProcessingEnabled() {
		return false
	}
	if err := policy.ValidateVariables(p, true); err != nil {
		return false
	}
	return true
}

func BuildKindSet(logger logr.Logger, policies ...kyvernov1.PolicyInterface) sets.String {
	kinds := sets.NewString()
	for _, policy := range policies {
		for _, rule := range autogen.ComputeRules(policy) {
			if rule.HasValidate() || rule.HasVerifyImages() {
				kinds.Insert(rule.MatchResources.GetKinds()...)
			}
		}
	}
	return kinds
}

func RemoveNonBackgroundPolicies(logger logr.Logger, policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var backgroundPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		if CanBackgroundProcess(logger, pol) {
			backgroundPolicies = append(backgroundPolicies, pol)
		}
	}
	return backgroundPolicies
}

func RemoveNonValidationPolicies(logger logr.Logger, policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var validationPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		spec := pol.GetSpec()
		if spec.HasVerifyImages() || spec.HasValidate() || spec.HasYAMLSignatureVerify() {
			validationPolicies = append(validationPolicies, pol)
		}
	}
	return validationPolicies
}
