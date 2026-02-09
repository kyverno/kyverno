package policies

import (
	"github.com/kyverno/kyverno/pkg/utils/slices"
)

type BackgroundAwarePolicy interface {
	BackgroundEnabled() bool
}

func RemoveNoneBackgroundPolicies[T BackgroundAwarePolicy](policies []T) []T {
	return slices.Filter(policies, func(vp T) bool {
		return vp.BackgroundEnabled()
	})
}
