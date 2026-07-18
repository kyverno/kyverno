package autogen

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Autogen(policy policiesv1beta1.ValidatingPolicyLike) (map[string]policiesv1beta1.ValidatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}
	spec := policy.GetValidatingPolicySpec()
	if !autogen.CanAutoGen(spec.MatchConstraints) {
		return nil, nil
	}
	actualControllers := autogen.AllConfigs
	if spec.AutogenConfiguration != nil &&
		spec.AutogenConfiguration.PodControllers != nil &&
		spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return generateRuleForControllers(*spec, actualControllers)
}

func RuleName(identifier string, index int) string {
	if identifier != "" {
		return "autogen-" + identifier
	}
	return fmt.Sprintf("autogen-validate-%d", index)
}

func ValidateUniqueIdentifiers(path *field.Path, identifiers []string) field.ErrorList {
	var allErrs field.ErrorList
	seen := sets.New[string]()
	for i, identifier := range identifiers {
		if identifier == "" {
			continue
		}
		if seen.Has(identifier) {
			allErrs = append(allErrs, field.Duplicate(path.Index(i).Child("identifier"), identifier))
			continue
		}
		seen.Insert(identifier)
	}
	return allErrs
}

func generateRuleForControllers(spec policiesv1beta1.ValidatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1beta1.ValidatingPolicyAutogen, error) {
	mapping := map[string][]policiesv1beta1.Target{}
	for config := range configs {
		if config := autogen.ConfigsMap[config]; config != nil {
			targets := mapping[config.ReplacementsRef]
			targets = append(targets, config.Target)
			mapping[config.ReplacementsRef] = targets
		}
	}
	rules := map[string]policiesv1beta1.ValidatingPolicyAutogen{}
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
		slices.SortFunc(targets, func(a, b policiesv1beta1.Target) int {
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
		rules[config] = policiesv1beta1.ValidatingPolicyAutogen{
			Targets: targets,
			Spec:    spec,
		}
	}
	return rules, nil
}
