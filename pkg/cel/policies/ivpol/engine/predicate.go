package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func MatchNames(names ...string) Predicate {
	if len(names) == 0 {
		return func(policiesv1beta1.ImageValidatingPolicyLike) bool { return true }
	}
	if len(names) == 1 {
		name := names[0]
		return func(policy policiesv1beta1.ImageValidatingPolicyLike) bool { return policy.GetName() == name }
	}
	namesSet := sets.New(names...)
	return func(policy policiesv1beta1.ImageValidatingPolicyLike) bool { return namesSet.Has(policy.GetName()) }
}
