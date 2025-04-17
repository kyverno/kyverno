package autogen

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func generateRuleForControllers(spec *policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string]) ([]policiesv1alpha1.AutogenRule, error) {
	mapping := map[string][]target{}
	for config := range configs {
		if config := configsMap[config]; config != nil {
			targets := mapping[config.replacementsRef]
			targets = append(targets, config.target)
			mapping[config.replacementsRef] = targets
		}
	}
	var rules []policiesv1alpha1.AutogenRule
	for _, replacements := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[replacements]
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		bytes, err := json.Marshal(policiesv1alpha1.ValidatingPolicySpec{
			MatchConstraints: createMatchConstraints(targets, operations),
			MatchConditions:  createMatchConditions(replacements, targets, spec.MatchConditions),
			Validations:      spec.Validations,
			AuditAnnotations: spec.AuditAnnotations,
			Variables:        spec.Variables,
		})
		if err != nil {
			return nil, err
		}
		bytes = updateFields(bytes, replacementsMap[replacements]...)
		var newSpec policiesv1alpha1.ValidatingPolicySpec
		if err := json.Unmarshal(bytes, &newSpec); err != nil {
			return nil, err
		}
		rules = append(rules, policiesv1alpha1.AutogenRule{
			MatchConstraints: newSpec.MatchConstraints,
			MatchConditions:  newSpec.MatchConditions,
			Validations:      newSpec.Validations,
			AuditAnnotation:  newSpec.AuditAnnotations,
			Variables:        newSpec.Variables,
		})
	}
	return rules, nil
}

func updateFields(data []byte, replacements ...replacement) []byte {
	for _, replacement := range replacements {
		data = bytes.ReplaceAll(data, []byte("object."+replacement.from), []byte("object."+replacement.to))
		data = bytes.ReplaceAll(data, []byte("oldObject."+replacement.from), []byte("oldObject."+replacement.to))
	}
	return data
}
