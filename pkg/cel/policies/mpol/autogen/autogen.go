package autogen

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	rootSpecRe            = regexp.MustCompile(`(^|request\.\??|[^\w.?])(object|oldObject|Object)((\.|\.\?)spec\b|\[\s*(\?)?\s*['"]spec['"]\s*\])`)
	rootMetaLabelsRe      = regexp.MustCompile(`(^|request\.\??|[^\w.?])(object|oldObject|Object)((\.|\.\?)metadata\b|\[\s*(\?)?\s*['"]metadata['"]\s*\])((\.|\.\?)labels\b|\[\s*(\?)?\s*['"]labels['"]\s*\])`)
	rootMetaAnnotationsRe = regexp.MustCompile(`(^|request\.\??|[^\w.?])(object|oldObject|Object)((\.|\.\?)metadata\b|\[\s*(\?)?\s*['"]metadata['"]\s*\])((\.|\.\?)annotations\b|\[\s*(\?)?\s*['"]annotations['"]\s*\])`)
)

// ExtractionReplacementsRef re-exports autogen.ExtractionReplacementsRef so
// callers of this package (e.g. the engine provider) don't need to import
// pkg/cel/autogen directly just to check it.
const ExtractionReplacementsRef = autogen.ExtractionReplacementsRef

func Autogen(policy policiesv1beta1.MutatingPolicyLike) (map[string]policiesv1beta1.MutatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}

	matchConstraints := policy.GetMatchConstraints()
	if !autogen.CanAutoGen(&matchConstraints) {
		return nil, nil
	}

	actualControllers := autogen.AllConfigs
	if policy.GetSpec().AutogenConfiguration != nil && policy.GetSpec().AutogenConfiguration.PodControllers != nil {
		pc := policy.GetSpec().AutogenConfiguration.PodControllers
		if pc.Enabled != nil && !*pc.Enabled {
			return nil, nil
		}
		if pc.Controllers != nil {
			actualControllers = sets.New(pc.Controllers...)
		}
	}
	return generateRuleForControllers(policy.GetSpec(), actualControllers)
}

func generateRuleForControllers(spec *policiesv1beta1.MutatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1beta1.MutatingPolicyAutogen, error) {
	mapping := map[string][]policiesv1beta1.Target{}
	for config := range configs {
		if config := autogen.ResolveConfig(config); config != nil {
			targets := mapping[config.ReplacementsRef]
			targets = append(targets, config.Target)
			mapping[config.ReplacementsRef] = targets
		}
	}
	rules := map[string]policiesv1beta1.MutatingPolicyAutogen{}
	for _, config := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[config]
		spec := spec.DeepCopy()
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		match := autogen.CreateMatchConstraints(targets, operations, spec.MatchConstraints.NamespaceSelector)
		spec.SetMatchConstraints(*match)

		// Extraction-mode targets (custom workload CRDs) have no fixed
		// "spec.template"-style path to rewrite expressions to - their pod
		// template is discovered at evaluation time instead - so
		// convertPodToTemplateExpression must not run for them. Without this
		// guard it would fall into its "default" case and rewrite these
		// expressions to the built-in flat shape, which is wrong for
		// whatever custom CRD this config actually targets.
		if config != autogen.ExtractionReplacementsRef {
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

		rules[config] = policiesv1beta1.MutatingPolicyAutogen{
			Targets: targets,
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

	expression = rootMetaLabelsRe.ReplaceAllStringFunc(expression, func(m string) string {
		sub := rootMetaLabelsRe.FindStringSubmatch(m)
		prefix := sub[1]
		root := sub[2]
		metaPart := sub[3]
		labelsPart := sub[6]

		metaDelim := "."
		if strings.Contains(metaPart, "?") {
			metaDelim = ".?"
		}
		labelsDelim := "."
		if strings.Contains(labelsPart, "?") {
			labelsDelim = ".?"
		}
		return prefix + root + metaDelim + metadataReplacement + labelsDelim + "labels"
	})

	expression = rootMetaAnnotationsRe.ReplaceAllStringFunc(expression, func(m string) string {
		sub := rootMetaAnnotationsRe.FindStringSubmatch(m)
		prefix := sub[1]
		root := sub[2]
		metaPart := sub[3]
		annPart := sub[6]

		metaDelim := "."
		if strings.Contains(metaPart, "?") {
			metaDelim = ".?"
		}
		annDelim := "."
		if strings.Contains(annPart, "?") {
			annDelim = ".?"
		}
		return prefix + root + metaDelim + metadataReplacement + annDelim + "annotations"
	})

	expression = rootSpecRe.ReplaceAllStringFunc(expression, func(m string) string {
		sub := rootSpecRe.FindStringSubmatch(m)
		prefix := sub[1]
		root := sub[2]
		specPart := sub[3]

		// Avoid double-replacement if already rewritten to template.
		idx := strings.Index(expression, m)
		if idx >= 0 {
			after := expression[idx+len(m):]
			if strings.HasPrefix(after, ".template") || strings.HasPrefix(after, ".?template") ||
				strings.HasPrefix(after, ".jobTemplate") || strings.HasPrefix(after, ".?jobTemplate") {
				return m
			}
		}

		specDelim := "."
		if strings.Contains(specPart, "?") {
			specDelim = ".?"
		}
		return prefix + root + specDelim + specReplacement
	})

	// Detect the root "Object{...}" constructor, tolerating optional whitespace
	// between "Object" and "{" (e.g. "Object {"), which is valid CEL.
	if rest, ok := strings.CutPrefix(strings.TrimSpace(expression), "Object"); ok {
		content := strings.TrimLeft(rest, " \t\r\n")
		content, ok = strings.CutPrefix(content, "{")
		if !ok {
			return expression
		}

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
