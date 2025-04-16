package autogen

import (
	"encoding/json"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetAutogenRulesImageVerify(policy *policiesv1alpha1.ImageValidatingPolicy) ([]*policiesv1alpha1.IvpolAutogen, error) {
	if policy == nil {
		return nil, nil
	}
	applyAutoGen := CanAutoGen(policy.Spec.MatchConstraints)
	if !applyAutoGen {
		return nil, nil
	}
	actualControllers := podControllers
	if policy.Spec.AutogenConfiguration != nil &&
		policy.Spec.AutogenConfiguration.PodControllers != nil &&
		policy.Spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(policy.Spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	genRules, err := autogenIvPols(policy, actualControllers)
	if err != nil {
		return nil, err
	}
	return genRules, nil
}

func autogenIvPols(ivpol *policiesv1alpha1.ImageValidatingPolicy, configs sets.Set[string]) ([]*policiesv1alpha1.IvpolAutogen, error) {
	genPolicy := func(resource autogencontroller, prefix string, configs sets.Set[string]) (*policiesv1alpha1.IvpolAutogen, error) {
		if ivpol == nil {
			return nil, nil
		}
		if len(configs) == 0 {
			return nil, nil
		}
		policy := &policiesv1alpha1.IvpolAutogen{
			Name: prefix + ivpol.GetName(),
			Spec: *ivpol.Spec.DeepCopy(),
		}
		// override match constraints for configs
		policy.Spec.MatchConstraints = createMatchConstraints(configs, ivpol.Spec.MatchConstraints.ResourceRules[0].Operations)
		// convert match conditions
		matchConditions, err := convertMatchConditions(policy.Spec.MatchConditions, resource)
		if err != nil {
			return nil, err
		}
		policy.Spec.MatchConditions = matchConditions
		// convert validations
		if bytes, err := json.Marshal(policy); err != nil {
			return nil, err
		} else {
			bytes = updateFields(bytes, resource)
			if err := json.Unmarshal(bytes, &policy); err != nil {
				return nil, err
			}
		}
		return policy, nil
	}
	ivpols := make([]*policiesv1alpha1.IvpolAutogen, 0, 2)
	cronjobs := sets.New("cronjobs")
	if configs.Has("cronjobs") {
		if p, err := genPolicy(CRONJOBS, "autogen-cronjobs-", cronjobs); err != nil {
			return nil, err
		} else if p != nil {
			ivpols = append(ivpols, p)
		}
	}
	if p, err := genPolicy(PODS, "autogen-", configs.Difference(cronjobs)); err != nil {
		return nil, err
	} else if p != nil {
		ivpols = append(ivpols, p)
	}
	return ivpols, nil
}
