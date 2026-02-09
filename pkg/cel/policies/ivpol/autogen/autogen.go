package autogen

import (
	"encoding/json"
	"maps"
	"slices"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Autogen(policy *policiesv1beta1.ImageValidatingPolicy) (map[string]policiesv1beta1.ImageValidatingPolicyAutogen, error) {
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
	return autogenIvPols(&policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: policy.ObjectMeta,
		Spec:       policy.Spec,
	}, actualControllers)
}

func AutogenNamespaced(policy *policiesv1beta1.NamespacedImageValidatingPolicy) (map[string]policiesv1beta1.ImageValidatingPolicyAutogen, error) {
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
	return autogenIvPols(&policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: policy.ObjectMeta,
		Spec:       policy.Spec,
	}, actualControllers)
}

func autogenIvPols(ivpol *policiesv1beta1.ImageValidatingPolicy, configs sets.Set[string]) (map[string]policiesv1beta1.ImageValidatingPolicyAutogen, error) {
	mapping := map[string][]policiesv1beta1.Target{}
	for config := range configs {
		if config := autogen.ConfigsMap[config]; config != nil {
			targets := mapping[config.ReplacementsRef]
			targets = append(targets, config.Target)
			mapping[config.ReplacementsRef] = targets
		}
	}
	spec := ivpol.Spec
	rules := map[string]policiesv1beta1.ImageValidatingPolicyAutogen{}
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

		rules[config] = policiesv1beta1.ImageValidatingPolicyAutogen{
			Targets: targets,
			Spec:    spec,
		}
	}
	return rules, nil
}
