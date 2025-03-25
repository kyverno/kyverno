package policy

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/utils/slices"
)

func RemoveNoneBackgroundValidatingPolicies(policies []v1alpha1.ValidatingPolicy) []v1alpha1.ValidatingPolicy {
	return slices.Filter(policies, func(vp v1alpha1.ValidatingPolicy) bool {
		return vp.Spec.BackgroundEnabled()
	})
}

func RemoveNoneBackgroundImageVerificationPolicies(policies []v1alpha1.ImageValidatingPolicy) []v1alpha1.ImageValidatingPolicy {
	return slices.Filter(policies, func(vp v1alpha1.ImageValidatingPolicy) bool {
		return vp.Spec.BackgroundEnabled()
	})
}
