package autogen

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ComputeRules(policy *policiesv1alpha1.ValidatingPolicy) ([]policiesv1alpha1.AutogenRule, error) {
	if policy == nil {
		return nil, nil
	}
	if !CanAutoGen(policy.GetSpec().MatchConstraints) {
		return nil, nil
	}
	actualControllers := allConfigs
	if policy.Spec.AutogenConfiguration != nil &&
		policy.Spec.AutogenConfiguration.PodControllers != nil &&
		policy.Spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(policy.Spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return generateRuleForControllers(policy.Spec.DeepCopy(), actualControllers)
}
