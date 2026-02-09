package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func MatchNames(names ...string) Predicate {
	if len(names) == 0 {
		return func(policiesv1beta1.ValidatingPolicyLike) bool { return true }
	}
	if len(names) == 1 {
		name := names[0]
		return func(policy policiesv1beta1.ValidatingPolicyLike) bool { return policy.GetName() == name }
	}
	namesSet := sets.New(names...)
	return func(policy policiesv1beta1.ValidatingPolicyLike) bool { return namesSet.Has(policy.GetName()) }
}

func ClusteredPolicy() Predicate {
	return func(policy policiesv1beta1.ValidatingPolicyLike) bool { return policy.GetNamespace() == "" }
}

func NamespacedPolicy(namespace string) Predicate {
	return func(policy policiesv1beta1.ValidatingPolicyLike) bool { return policy.GetNamespace() == namespace }
}

func And(conditions ...Predicate) Predicate {
	return func(policy policiesv1beta1.ValidatingPolicyLike) bool {
		for _, condition := range conditions {
			if !condition(policy) {
				return false
			}
		}
		return true
	}
}
