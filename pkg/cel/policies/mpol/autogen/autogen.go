package autogen

import (
	"cmp"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Autogen(policy *policiesv1alpha1.MutatingPolicy) (map[string]policiesv1alpha1.MutatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}

	matchConstraints := policy.GetMatchConstraints()
	if !autogen.CanAutoGen(&matchConstraints) {
		return nil, nil
	}

	actualControllers := autogen.AllConfigs
	if policy.Spec.AutogenConfiguration != nil &&
		policy.Spec.AutogenConfiguration.PodControllers != nil &&
		policy.Spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(policy.Spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return generateRuleForControllers(&policy.Spec, actualControllers)
}

func generateRuleForControllers(spec *policiesv1alpha1.MutatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1alpha1.MutatingPolicyAutogen, error) {
	mapping := map[string][]policiesv1alpha1.Target{}
	for config := range configs {
		if config := autogen.ConfigsMap[config]; config != nil {
			targets := mapping[config.ReplacementsRef]
			targets = append(targets, config.Target)
			mapping[config.ReplacementsRef] = targets
		}
	}
	rules := map[string]policiesv1alpha1.MutatingPolicyAutogen{}
	for _, config := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[config]
		spec := spec.DeepCopy()
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		match := autogen.CreateMatchConstraints(targets, operations)
		spec.SetMatchConstraints(*match)
		bytes, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}

		var specMap map[string]interface{}
		if err := json.Unmarshal(bytes, &specMap); err != nil {
			return nil, err
		}

		if mutations, exists := specMap["mutations"]; exists {
			if mutationsList, ok := mutations.([]interface{}); ok {
				for _, mutation := range mutationsList {
					if mutationMap, ok := mutation.(map[string]interface{}); ok {
						if applyConfig, exists := mutationMap["applyConfiguration"]; exists {
							if applyConfigMap, ok := applyConfig.(map[string]interface{}); ok {
								if expression, exists := applyConfigMap["expression"]; exists {
									if exprStr, ok := expression.(string); ok {
										// Convert pod pattern to template pattern
										convertedExpr := convertPodToTemplateExpression(exprStr, config)
										applyConfigMap["expression"] = convertedExpr
									}
								}
							}
						}
					}
				}
			}
		}

		bytes, err = json.Marshal(specMap)
		if err != nil {
			return nil, err
		}
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
		rules[config] = policiesv1alpha1.MutatingPolicyAutogen{
			Targets: targets,
			Spec:    spec,
		}
	}
	return rules, nil
}

// convertPodToTemplateExpression converts pod mutation expressions to template expressions
func convertPodToTemplateExpression(expression string, config string) string {
	var specReplacement string
	switch config {
	case "cronjobs":
		specReplacement = "spec.jobTemplate.spec.template.spec"
	default:
		specReplacement = "spec.template.spec"
	}

	expression = strings.ReplaceAll(expression, "object.spec", "object."+specReplacement)
	expression = strings.ReplaceAll(expression, "Object.spec", "Object."+specReplacement)

	if strings.HasPrefix(strings.TrimSpace(expression), "Object{") {
		content := strings.TrimSpace(expression)
		content = strings.TrimPrefix(content, "Object{")

		braceCount := 1
		endIndex := 0
		for i, char := range content {
			if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount == 0 {
					endIndex = i
					break
				}
			}
		}

		if endIndex > 0 {
			innerContent := content[:endIndex]
			remainingContent := content[endIndex+1:]

			var wrapper string
			var closingBraces string
			switch config {
			case "cronjobs":
				wrapper = "Object{spec: Object.spec{jobTemplate: Object.spec.jobTemplate{spec: Object.spec.jobTemplate.spec{template: Object.spec.jobTemplate.spec.template{"
				closingBraces = "}}}}}"
			default:
				wrapper = "Object{spec: Object.spec{template: Object.spec.template{"
				closingBraces = "}}}"
			}

			return wrapper + innerContent + closingBraces + remainingContent
		}
	}

	return expression
}
