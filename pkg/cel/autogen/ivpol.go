package autogen

import (
	"encoding/json"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetAutogenRulesImageVerify(policy *policiesv1alpha1.ImageValidatingPolicy) ([]policiesv1alpha1.IvpolAutogen, error) {
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
	return autogenIvPols(policy, actualControllers)
}

func autogenIvPols(ivpol *policiesv1alpha1.ImageValidatingPolicy, configs sets.Set[string]) ([]policiesv1alpha1.IvpolAutogen, error) {
	mapping := map[*replacements][]target{}
	for config := range configs {
		if config := builtins[config]; config != nil {
			targets := mapping[config.replacements]
			targets = append(targets, config.target)
			mapping[config.replacements] = targets
		}
	}
	var rules []policiesv1alpha1.IvpolAutogen
	for replacements, targets := range mapping {
		spec := ivpol.Spec.DeepCopy()
		operations := ivpol.Spec.MatchConstraints.ResourceRules[0].Operations
		spec.MatchConstraints = createMatchConstraints(targets, operations)
		spec.MatchConditions = createMatchConditions(targets, ivpol.Spec.MatchConditions)
		if bytes, err := json.Marshal(spec); err != nil {
			return nil, err
		} else {
			bytes = updateFields(bytes, replacements.entries...)
			if err := json.Unmarshal(bytes, spec); err != nil {
				return nil, err
			}
		}
		rules = append(rules, policiesv1alpha1.IvpolAutogen{
			Name: ivpol.GetName(),
			Spec: *spec,
		})
	}
	return rules, nil
}
