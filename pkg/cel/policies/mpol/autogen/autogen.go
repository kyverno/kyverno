package autogen

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Autogen(policy *policiesv1alpha1.MutatingPolicy) (map[string]policiesv1alpha1.MutatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}

	spec := policy.GetMutatingPolicySpec()

	matchConstraints := policy.GetMatchConstraints()
	if !autogen.CanAutoGen(&matchConstraints) {
		return nil, nil
	}

	actualControllers := autogen.AllConfigs
	if spec.AutogenConfiguration != nil &&
		spec.AutogenConfiguration.PodControllers != nil &&
		spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return generateRuleForControllers(spec, actualControllers)
}

func AutogenNamespaced(policy *policiesv1alpha1.NamespacedMutatingPolicy) (map[string]policiesv1alpha1.MutatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}

	spec := policy.GetMutatingPolicySpec()
	matchConstraints := policy.GetMatchConstraints()
	if !autogen.CanAutoGen(&matchConstraints) {
		return nil, nil
	}
	
	actualControllers := autogen.AllConfigs
	if spec.AutogenConfiguration != nil &&
		spec.AutogenConfiguration.PodControllers != nil &&
		spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return generateRuleForControllers(spec, actualControllers)
}	

func generateRuleForControllers(spec *policiesv1alpha1.MutatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1alpha1.MutatingPolicyAutogen, error) {
	mapping := map[string][]policiesv1beta1.Target{}
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

		for i := range spec.MatchConditions {
			if spec.MatchConditions[i].Expression != "" {
				convertedExpr := convertPodToTemplateExpression(spec.MatchConditions[i].Expression, config)
				spec.MatchConditions[i].Expression = convertedExpr
			}
		}

		for i := range spec.Mutations {
			if spec.Mutations[i].ApplyConfiguration != nil && spec.Mutations[i].ApplyConfiguration.Expression != "" {
				convertedExpr := convertPodToTemplateExpression(spec.Mutations[i].ApplyConfiguration.Expression, config)
				spec.Mutations[i].ApplyConfiguration.Expression = convertedExpr
			}
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

		t := make([]policiesv1alpha1.Target, 0, len(targets))
		for _, target := range targets {
			t = append(t, policiesv1alpha1.Target(target))
		}

		rules[config] = policiesv1alpha1.MutatingPolicyAutogen{
			Targets: t,
			Spec:    spec,
		}
	}
	return rules, nil
}

// convertPodToTemplateExpression converts pod mutation expressions to template expressions
func convertPodToTemplateExpression(expression string, config string) string {
	var specReplacement string
	var metadataReplacement string

	switch config {
	case "cronjobs":
		specReplacement = "spec.jobTemplate.spec.template.spec"
		metadataReplacement = "spec.jobTemplate.spec.template.metadata"
	default:
		specReplacement = "spec.template.spec"
		metadataReplacement = "spec.template.metadata"
	}

	expression = strings.ReplaceAll(expression, "object.spec", "object."+specReplacement)
	expression = strings.ReplaceAll(expression, "Object.spec", "Object."+specReplacement)

	expression = strings.ReplaceAll(expression, "object.metadata.labels", "object."+metadataReplacement+".labels")
	expression = strings.ReplaceAll(expression, "Object.metadata.labels", "Object."+metadataReplacement+".labels")
	expression = strings.ReplaceAll(expression, "object.metadata.annotations", "object."+metadataReplacement+".annotations")
	expression = strings.ReplaceAll(expression, "Object.metadata.annotations", "Object."+metadataReplacement+".annotations")

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
