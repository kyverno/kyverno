package autogen

import (
	"encoding/json"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetAutogenRulesImageVerify(policy *policiesv1alpha1.ImageValidatingPolicy) ([]*policiesv1alpha1.IvpolAutogen, error) {
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

func autogenIvPols(ivpol *policiesv1alpha1.ImageValidatingPolicy, controllerSet sets.Set[string]) ([]*policiesv1alpha1.IvpolAutogen, error) {
	genPolicy := func(resource autogencontroller, controllers string) (policy *policiesv1alpha1.IvpolAutogen, err error) {
		if len(controllers) == 0 {
			return nil, nil
		}

		if ivpol == nil {
			return nil, nil
		}

		policy = &policiesv1alpha1.IvpolAutogen{}
		copied := ivpol.DeepCopy()
		policy.Spec = copied.Spec
		if controllers == "cronjobs" {
			policy.Name = "autogen-cronjobs-" + ivpol.GetName()
		} else {
			policy.Name = "autogen-" + ivpol.GetName()
		}
		operations := ivpol.Spec.MatchConstraints.ResourceRules[0].Operations
		// create a resource rule for pod controllers
		policy.Spec.MatchConstraints = createMatchConstraints(controllers, operations)

		// convert match conditions
		policy.Spec.MatchConditions, err = convertMatchConditions(policy.Spec.MatchConditions, resource)
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

	ivpols := make([]*policiesv1alpha1.IvpolAutogen, 0)
	if controllerSet.Has("cronjobs") {
		p, err := genPolicy(CRONJOBS, "cronjobs")
		if err != nil {
			return nil, err
		}
		if p != nil {
			ivpols = append(ivpols, p)
		}
	}

	controllerSetCopied := controllerSet.Clone()
	p, err := genPolicy(PODS, strings.Join(sets.List(controllerSetCopied.Delete("cronjobs")), ","))
	if err != nil {
		return nil, err
	}
	if p != nil {
		ivpols = append(ivpols, p)
	}
	return ivpols, nil
}
