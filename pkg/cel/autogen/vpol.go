package autogen

import (
	"encoding/json"
	"maps"
	"slices"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ValidatingPolicy(policy *policiesv1alpha1.ValidatingPolicy) (map[string]policiesv1alpha1.ValidatingPolicyAutogen, error) {
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
	return generateRuleForControllers(policy.Spec, actualControllers)
}

func generateRuleForControllers(spec policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1alpha1.ValidatingPolicyAutogen, error) {
	mapping := map[string][]target{}
	for config := range configs {
		if config := configsMap[config]; config != nil {
			targets := mapping[config.replacementsRef]
			targets = append(targets, config.target)
			mapping[config.replacementsRef] = targets
		}
	}
	rules := map[string]policiesv1alpha1.ValidatingPolicyAutogen{}
	for _, replacements := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[replacements]
		spec := spec.DeepCopy()
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		spec.MatchConstraints = createMatchConstraints(targets, operations)
		spec.MatchConditions = createMatchConditions(replacements, targets, spec.MatchConditions)
		bytes, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}
		bytes = updateFields(bytes, replacementsMap[replacements]...)
		if err := json.Unmarshal(bytes, spec); err != nil {
			return nil, err
		}
		rules[replacements] = policiesv1alpha1.ValidatingPolicyAutogen{
			Spec: spec,
		}
	}
	return rules, nil
}
