package autogen

import (
	"encoding/json"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetAutogenRulesImageVerify(policy *policiesv1alpha1.ImageVerificationPolicy) ([]*policiesv1alpha1.ImageVerificationPolicy, error) {
	if policy == nil {
		return nil, nil
	}

	applyAutoGen, desiredControllers := CanAutoGen(policy.GetSpec().MatchConstraints)
	if !applyAutoGen {
		return nil, nil
	}

	var actualControllers sets.Set[string]
	ann := policy.GetAnnotations()
	actualControllersString, ok := ann[kyverno.AnnotationAutogenControllers]
	if !ok {
		actualControllers = desiredControllers
	} else {
		actualControllers = sets.New(strings.Split(actualControllersString, ",")...)
	}

	genRules, err := autogenIvPols(policy, actualControllers)
	if err != nil {
		return nil, err
	}

	return genRules, nil
}

func autogenIvPols(ivpol *policiesv1alpha1.ImageVerificationPolicy, controllerSet sets.Set[string]) ([]*policiesv1alpha1.ImageVerificationPolicy, error) {
	genPolicy := func(resource autogencontroller, controllers string) (*policiesv1alpha1.ImageVerificationPolicy, error) {
		if len(controllers) == 0 {
			return nil, nil
		}

		if ivpol == nil {
			return nil, nil
		}
		var err error
		policy := ivpol.DeepCopy()
		if controllers == "cronjobs" {
			policy.Name = "autogen-cronjobs-" + policy.Name
		} else {
			policy.Name = "autogen-" + policy.Name
		}
		operations := ivpol.Spec.MatchConstraints.ResourceRules[0].Operations
		// create a resource rule for pod controllers
		policy.Spec.MatchConstraints = createMatchConstraints(controllers, operations)

		// convert match conditions
		policy.Spec.MatchConditions, err = convertMatchconditions(ivpol.Spec.MatchConditions, resource)
		if err != nil {
			return nil, err
		}

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

	ivpols := make([]*policiesv1alpha1.ImageVerificationPolicy, 0)
	if controllerSet.Has("cronjobs") {
		p, err := genPolicy(CRONJOBS, "cronjobs")
		if err != nil {
			return nil, err
		}
		if p != nil {
			ivpols = append(ivpols, p)
		}
	}

	p, err := genPolicy(PODS, strings.Join(sets.List(controllerSet.Delete("cronjobs")), ","))
	if err != nil {
		return nil, err
	}
	if p != nil {
		ivpols = append(ivpols, p)
	}
	return ivpols, nil
}
