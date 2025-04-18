package autogen

import (
	"encoding/json"
	"maps"
	"slices"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetAutogenRulesImageVerify(policy *policiesv1alpha1.ImageValidatingPolicy) (map[string]policiesv1alpha1.ImageValidatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}
	if !CanAutoGen(policy.Spec.MatchConstraints) {
		return nil, nil
	}
	actualControllers := allConfigs
	if policy.Spec.AutogenConfiguration != nil &&
		policy.Spec.AutogenConfiguration.PodControllers != nil &&
		policy.Spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(policy.Spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return autogenIvPols(policy.Spec, actualControllers)
}

func autogenIvPols(spec policiesv1alpha1.ImageValidatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1alpha1.ImageValidatingPolicyAutogen, error) {
	mapping := map[string][]target{}
	for config := range configs {
		if config := configsMap[config]; config != nil {
			targets := mapping[config.replacementsRef]
			targets = append(targets, config.target)
			mapping[config.replacementsRef] = targets
		}
	}
	rules := map[string]policiesv1alpha1.ImageValidatingPolicyAutogen{}
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
		rules[replacements] = policiesv1alpha1.ImageValidatingPolicyAutogen{
			Spec: spec,
		}
	}
	return rules, nil
}
