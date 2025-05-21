package autogen

import (
	"cmp"
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
	for _, config := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[config]
		spec := spec.DeepCopy()
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		spec.MatchConstraints = autogen.CreateMatchConstraints(targets, operations)
		bytes, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}
		bytes = autogen.Apply(bytes, autogen.ReplacementsMap[config]...)
		if err := json.Unmarshal(bytes, spec); err != nil {
			return nil, err
		}
		slices.SortFunc(targets, func(a, b policiesv1alpha1.Target) int {
			if x := cmp.Compare(a.Group, b.Group); x != 0 {
				return x
			}
			if x := cmp.Compare(a.Version, b.Version); x != 0 {
				return x
			}
			if x := cmp.Compare(a.Resource, b.Resource); x != 0 {
				return x
			}
			if x := cmp.Compare(a.Kind, b.Kind); x != 0 {
				return x
			}
			return 0
		})
		rules[config] = policiesv1alpha1.ValidatingPolicyAutogen{
			Targets: targets,
			Spec:    spec,
		}
	}
	return rules, nil
}
