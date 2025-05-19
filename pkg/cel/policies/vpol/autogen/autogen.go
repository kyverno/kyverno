package autogen

import (
	"encoding/json"
	"maps"
	"slices"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Autogen(policy *policiesv1alpha1.ValidatingPolicy) (map[string]policiesv1alpha1.ValidatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}
	if !autogen.CanAutoGen(policy.Spec.MatchConstraints) {
		return nil, nil
	}
	actualControllers := autogen.AllConfigs
	if policy.Spec.AutogenConfiguration != nil &&
		policy.Spec.AutogenConfiguration.PodControllers != nil &&
		policy.Spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(policy.Spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return generateRuleForControllers(policy.Spec, actualControllers)
}

func generateRuleForControllers(spec policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1alpha1.ValidatingPolicyAutogen, error) {
	mapping := map[string][]policiesv1alpha1.Target{}
	for config := range configs {
		if config := autogen.ConfigsMap[config]; config != nil {
			targets := mapping[config.ReplacementsRef]
			targets = append(targets, config.Target)
			mapping[config.ReplacementsRef] = targets
		}
	}
	rules := map[string]policiesv1alpha1.ValidatingPolicyAutogen{}
	for _, replacements := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[replacements]
		spec := spec.DeepCopy()
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		spec.MatchConstraints = autogen.CreateMatchConstraints(targets, operations)
		// spec.MatchConditions = autogen.CreateMatchConditions(replacements, targets, spec.MatchConditions)
		bytes, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}
		bytes = autogen.Apply(bytes, autogen.ReplacementsMap[replacements]...)
		if err := json.Unmarshal(bytes, spec); err != nil {
			return nil, err
		}
		rules[replacements] = policiesv1alpha1.ValidatingPolicyAutogen{
			Spec: spec,
		}
	}
	return rules, nil
}
