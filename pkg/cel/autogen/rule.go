package autogen

import (
	"bytes"
	"encoding/json"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func generateRuleForControllers(spec *policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string]) ([]policiesv1alpha1.AutogenRule, error) {
	mapping := map[*replacements][]target{}
	for config := range configs {
		if config := builtins[config]; config != nil {
			targets := mapping[config.replacements]
			targets = append(targets, config.target)
			mapping[config.replacements] = targets
		}
	}
	var rules []policiesv1alpha1.AutogenRule
	for replacements, targets := range mapping {
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		newSpec := &policiesv1alpha1.ValidatingPolicySpec{
			MatchConstraints: createMatchConstraints(targets, operations),
			MatchConditions:  createMatchConditions(targets, spec.MatchConditions),
			Validations:      spec.Validations,
			AuditAnnotations: spec.AuditAnnotations,
			Variables:        spec.Variables,
		}
		if bytes, err := json.Marshal(newSpec); err != nil {
			return nil, err
		} else {
			bytes = updateFields(bytes, replacements.entries...)
			if err := json.Unmarshal(bytes, &newSpec); err != nil {
				return nil, err
			}
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
