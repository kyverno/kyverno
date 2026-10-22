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
	specRe                = regexp.MustCompile(`\b(object|oldObject|Object)\.spec`)
	metadataLabelsRe      = regexp.MustCompile(`\b(object|oldObject|Object)\.metadata\.labels`)
	metadataAnnotationsRe = regexp.MustCompile(`\b(object|oldObject|Object)\.metadata\.annotations`)
	metadataTypeRe        = regexp.MustCompile(`\bObject\.metadata\b`)
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
	if policy.GetSpec().AutogenConfiguration != nil &&
		policy.GetSpec().AutogenConfiguration.PodControllers != nil &&
		policy.GetSpec().AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(policy.GetSpec().AutogenConfiguration.PodControllers.Controllers...)
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

	expression = rewriteOutsideLiterals(expression, func(code string) string {
		code = specRe.ReplaceAllString(code, "${1}."+specReplacement)
		code = metadataLabelsRe.ReplaceAllString(code, "${1}."+metadataReplacement+".labels")
		code = metadataAnnotationsRe.ReplaceAllString(code, "${1}."+metadataReplacement+".annotations")
		return metadataTypeRe.ReplaceAllString(code, "Object."+metadataReplacement)
	})

	// Detect the root "Object{...}" constructor, tolerating optional whitespace
	// between "Object" and "{" (e.g. "Object {"), which is valid CEL.
	if rest, ok := strings.CutPrefix(strings.TrimSpace(expression), "Object"); ok {
		content := strings.TrimLeft(rest, " \t\r\n")
		content, ok = strings.CutPrefix(content, "{")
		if !ok {
			return expression
		}

		endIndex := findMatchingBrace(content)
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

// rewriteOutsideLiterals applies rewrite to the code segments of a CEL expression
// and copies string/bytes literals and comments through unchanged. Keeping literals
// out of the regex rewrites prevents payload text such as "Object.metadata" from
// being changed along with the actual type reference.
func rewriteOutsideLiterals(expression string, rewrite func(string) string) string {
	var builder strings.Builder
	start := 0
	for i := 0; i < len(expression); {
		if end, ok := commentEnd(expression, i); ok {
			if start < i {
				builder.WriteString(rewrite(expression[start:i]))
			}
			builder.WriteString(expression[i:end])
			i = end
			start = end
			continue
		}
		if literalStart, end, ok := stringLiteralAt(expression, i); ok {
			if start < literalStart {
				builder.WriteString(rewrite(expression[start:literalStart]))
			}
			builder.WriteString(expression[literalStart:end])
			i = end
			start = end
			continue
		}
		i++
	}
	if start < len(expression) {
		builder.WriteString(rewrite(expression[start:]))
	}
	return builder.String()
}

// findMatchingBrace returns the index of the brace that closes the root Object
// constructor, ignoring braces inside literals and comments. It returns 0 when
// there is no matching brace.
func findMatchingBrace(content string) int {
	depth := 1
	for i := 0; i < len(content); {
		if end, ok := commentEnd(content, i); ok {
			i = end
			continue
		}
		if _, end, ok := stringLiteralAt(content, i); ok {
			i = end
			continue
		}
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return 0
}

// commentEnd returns the end offset of the CEL line comment starting at i.
// Comments run from "//" to the end of the line.
func commentEnd(expression string, i int) (int, bool) {
	if i+1 >= len(expression) || expression[i] != '/' || expression[i+1] != '/' {
		return 0, false
	}
	if offset := strings.IndexByte(expression[i+2:], '\n'); offset >= 0 {
		return i + 2 + offset, true
	}
	return len(expression), true
}

// stringLiteralAt returns the start and end offsets of the CEL string or bytes
// literal whose opening quote is at quote. The start offset includes the optional
// b/B and r/R prefixes. CEL supports single, double and triple quoted literals,
// with escapes in non-raw literals and no escape handling in raw literals.
func stringLiteralAt(expression string, quote int) (int, int, bool) {
	if expression[quote] != '"' && expression[quote] != '\'' {
		return 0, 0, false
	}

	start := quote
	raw := false
	if quote > 0 {
		switch expression[quote-1] {
		case 'r', 'R':
			raw = true
			start = quote - 1
			if start > 0 && (expression[start-1] == 'b' || expression[start-1] == 'B') {
				start--
			}
		case 'b', 'B':
			start = quote - 1
		}
		if start < quote && start > 0 && isIdentifierChar(expression[start-1]) {
			// The candidate prefix is the tail of an identifier, not a literal prefix.
			start = quote
			raw = false
		}
	}

	quoteChar := expression[quote]
	triple := quote+2 < len(expression) && expression[quote+1] == quoteChar && expression[quote+2] == quoteChar
	i := quote + 1
	if triple {
		i = quote + 3
	}
	for i < len(expression) {
		if !raw && expression[i] == '\\' {
			// Skip the escaped character. Multi-character escapes such as \u0041
			// are still inside the literal, so skipping only the next byte is safe.
			i += 2
			continue
		}
		if triple {
			if i+2 < len(expression) && expression[i] == quoteChar && expression[i+1] == quoteChar && expression[i+2] == quoteChar {
				return start, i + 3, true
			}
		} else if expression[i] == quoteChar {
			return start, i + 1, true
		}
		i++
	}
	return start, len(expression), true
}

// isIdentifierChar reports whether c can be part of a CEL identifier.
func isIdentifierChar(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
