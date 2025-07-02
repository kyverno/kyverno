package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func MatchNames(names ...string) Predicate {
	if len(names) == 0 {
		return func(policiesv1alpha1.MutatingPolicy) bool { return true }
	}
	if len(names) == 1 {
		name := names[0]
		return func(policy policiesv1alpha1.MutatingPolicy) bool { return policy.Name == name }
	}
	namesSet := sets.New(names...)
	return func(policy policiesv1alpha1.MutatingPolicy) bool { return namesSet.Has(policy.Name) }
}
