package autogen

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ExtractionReplacementsRef re-exports autogen.ExtractionReplacementsRef so
// callers of this package (e.g. the engine provider) don't need to import
// pkg/cel/autogen directly just to check it.
const ExtractionReplacementsRef = autogen.ExtractionReplacementsRef

// RewriteExceptions rewrites PolicyException match conditions and validations using the same
// spec field-path replacements applied to the policy spec for the given autogen
// config, delegating to the unified implementation in pkg/cel/autogen.
func RewriteExceptions(exceptions []*policiesv1beta1.PolicyException, config string) ([]*policiesv1beta1.PolicyException, error) {
	return autogen.RewriteExceptions(exceptions, config)
}

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
	specs := map[string]*policiesv1beta1.MutatingPolicySpec{}
	for name := range configs {
		config := autogen.ResolveMutatingConfig(name)
		if config == nil {
			continue
		}
		// A name ConfigsMap does not already map is only on the built-in path
		// because ResolveMutatingConfig remapped its explicit canonical form
		// (e.g. "deployments.v1.apps") to that built-in config.
		remapped := autogen.ConfigsMap[name] == nil
		generated := spec.DeepCopy()
		if config.ReplacementsRef != autogen.ExtractionReplacementsRef && !convertPodToTemplate(generated, config.ReplacementsRef, remapped) {
			// Not every mutation can be rewritten to the pod template safely.
			// Keep this target in extraction mode instead of activating the
			// built-in path: the engine skips extraction-mode mutation, which
			// is correct, whereas a Pod-shaped mutation applied to the parent
			// workload would mutate the wrong object at the wrong path.
			config = &autogen.Config{Target: config.Target, ReplacementsRef: autogen.ExtractionReplacementsRef}
			generated = spec.DeepCopy()
		}
		mapping[config.ReplacementsRef] = append(mapping[config.ReplacementsRef], config.Target)
		specs[config.ReplacementsRef] = generated
	}
	rules := map[string]policiesv1beta1.MutatingPolicyAutogen{}
	for _, config := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[config]
		spec := specs[config]
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		match := autogen.CreateMatchConstraints(targets, operations, spec.MatchConstraints.NamespaceSelector)
		spec.SetMatchConstraints(*match)

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

// convertPodToTemplate rewrites a spec's expressions from their pod-shaped form
// to config's pod template path, reporting false when a mutation cannot be
// converted safely. Variables and match conditions are only rewritten as
// selectors: they are evaluated against the workload itself, not against a
// mutation object, so there is no constructor to wrap.
//
// remappedToBuiltIn reports that the target is on the built-in path only through
// ResolveMutatingConfig's explicit canonical form remapping. JSONPatch path and
// from pointers are not rewritten for the pod template yet, so such a target
// must stay in extraction mode: a Pod patch to /metadata/labels/checked would
// otherwise label the workload itself. A name that was already built-in keeps
// its existing JSONPatch behaviour, which this change does not alter.
func convertPodToTemplate(spec *policiesv1beta1.MutatingPolicySpec, config string, remappedToBuiltIn bool) bool {
	for i := range spec.Variables {
		if spec.Variables[i].Expression != "" {
			expression := autogen.ApplyMutatingCEL([]byte(spec.Variables[i].Expression), config)
			// A variable can hold a pod object constructor that the mutation then
			// places under the template (e.g. Object{metadata: variables.m}), so
			// its constructor type names need the same qualification as the
			// mutation's own. Otherwise the patch is rejected with a type mismatch.
			spec.Variables[i].Expression = string(autogen.QualifyMutatingMetadataType(expression, config))
		}
	}

	for i := range spec.MatchConditions {
		if spec.MatchConditions[i].Expression != "" {
			spec.MatchConditions[i].Expression = string(autogen.ApplyMutatingCEL([]byte(spec.MatchConditions[i].Expression), config))
		}
	}

	for i := range spec.Mutations {
		mutation := &spec.Mutations[i]
		if mutation.JSONPatch != nil {
			if remappedToBuiltIn {
				return false
			}
			continue
		}
		if mutation.ApplyConfiguration != nil && mutation.ApplyConfiguration.Expression != "" {
			convertedExpr, ok := convertPodToTemplateExpression(mutation.ApplyConfiguration.Expression, config)
			if !ok {
				return false
			}
			mutation.ApplyConfiguration.Expression = convertedExpr
		}
	}
	return true
}

// convertPodToTemplateExpression converts pod mutation expressions to template
// expressions, reporting false when the expression's shape cannot be rewritten
// to the template without guessing.
//
// Supported shapes are the ones whose result is a pod object constructor: a
// bare Object{...} (optionally followed by comments), and a top-level
// conditional (a ? b : c) whose branches are themselves supported, which is
// rewritten branch by branch. Any other expression - a call, a selector, a
// variable reference - reports false, and the caller keeps that target in
// extraction mode rather than splicing a wrapper into an expression it does not
// understand.
func convertPodToTemplateExpression(expression string, config string) (string, bool) {
	converted := string(autogen.ApplyMutatingCEL([]byte(expression), config))
	return wrapPodTemplateExpression(converted, config)
}

// wrapPodTemplateExpression wraps every supported branch of expression in the
// pod template constructors for config.
func wrapPodTemplateExpression(expression string, config string) (string, bool) {
	if wrapped, ok := wrapPodObjectConstructor(expression, config); ok {
		return wrapped, true
	}
	if condition, whenTrue, whenFalse, ok := splitTopLevelConditional(expression); ok {
		wrappedTrue, okTrue := wrapPodTemplateExpression(whenTrue, config)
		wrappedFalse, okFalse := wrapPodTemplateExpression(whenFalse, config)
		if okTrue && okFalse {
			return condition + "?" + wrappedTrue + ":" + wrappedFalse, true
		}
	}
	return expression, false
}

// wrapPodObjectConstructor wraps a whole Object{...} constructor, tolerating
// optional whitespace between "Object" and "{" (e.g. "Object {") and trailing
// comments. The constructor's metadata type names are qualified for the
// template at the same time.
func wrapPodObjectConstructor(expression string, config string) (string, bool) {
	trimmed := strings.TrimSpace(expression)
	leading := expression[:len(expression)-len(strings.TrimLeft(expression, " \t\r\n"))]
	trailing := expression[len(strings.TrimRight(expression, " \t\r\n")):]

	rest, ok := strings.CutPrefix(trimmed, "Object")
	if !ok {
		return "", false
	}
	content, ok := strings.CutPrefix(strings.TrimLeft(rest, " \t\r\n"), "{")
	if !ok {
		return "", false
	}
	endIndex, ok := autogen.ScanBalancedBraces(content)
	if !ok {
		return "", false
	}
	// Anything after the constructor must be a comment: a larger expression
	// (a concatenation, a method call, a further selector) is not a shape this
	// conversion knows how to rewrite.
	remaining := content[endIndex+1:]
	if !isCommentOrWhitespace(remaining) {
		return "", false
	}

	innerContent := string(autogen.QualifyMutatingMetadataType([]byte(content[:endIndex]), config))
	wrapper, closingBraces := podTemplateWrapper(config)
	return leading + wrapper + innerContent + closingBraces + remaining + trailing, true
}

// podTemplateWrapper returns the constructor prefix and closing braces that nest
// a pod object under config's pod template.
func podTemplateWrapper(config string) (string, string) {
	if config == autogen.AutogenCronjobs {
		return "Object{spec: Object.spec{jobTemplate: Object.spec.jobTemplate{spec: Object.spec.jobTemplate.spec{template: Object.spec.jobTemplate.spec.template{", "}}}}}"
	}
	return "Object{spec: Object.spec{template: Object.spec.template{", "}}}"
}

// isCommentOrWhitespace reports whether s contains only CEL comments and
// whitespace.
func isCommentOrWhitespace(s string) bool {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		switch {
		case s == "":
			return true
		case strings.HasPrefix(s, "//"):
			newline := strings.IndexByte(s, '\n')
			if newline < 0 {
				return true
			}
			s = s[newline+1:]
		case strings.HasPrefix(s, "/*"):
			end := strings.Index(s, "*/")
			if end < 0 {
				return false
			}
			s = s[end+2:]
		default:
			return false
		}
	}
}

// splitTopLevelConditional splits a top-level CEL conditional (a ? b : c) into
// its condition and two branches. It reports false when expression is not such a
// conditional. Optional selectors (.?field) and the colon of a map literal are
// not mistaken for a ternary: '?' only counts at brace, bracket and parenthesis
// depth zero and when it does not follow a dot, and a ':' only splits when it
// closes the outermost '?'.
func splitTopLevelConditional(expression string) (string, string, string, bool) {
	depth := 0
	var questions []int
	for i := 0; i < len(expression); {
		if next, ok := skipCELLiteral(expression, i); ok {
			i = next
			continue
		}
		if strings.HasPrefix(expression[i:], "//") {
			newline := strings.IndexByte(expression[i:], '\n')
			if newline < 0 {
				return "", "", "", false
			}
			i += newline + 1
			continue
		}
		if strings.HasPrefix(expression[i:], "/*") {
			end := strings.Index(expression[i:], "*/")
			if end < 0 {
				return "", "", "", false
			}
			i += end + 2
			continue
		}
		switch expression[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '?':
			if depth == 0 && !followsDot(expression, i) {
				questions = append(questions, i)
			}
		case ':':
			if depth == 0 && len(questions) > 0 {
				open := questions[len(questions)-1]
				questions = questions[:len(questions)-1]
				if len(questions) == 0 {
					return expression[:open], expression[open+1 : i], expression[i+1:], true
				}
			}
		}
		i++
	}
	return "", "", "", false
}

// skipCELLiteral returns the index just past the CEL string literal starting at
// i, and false when no literal starts there. It follows the same literal rules
// as autogen.ScanBalancedBraces: raw (r/R) prefixes, triple-quoted and backtick
// strings, and backslash escapes in the interpreted forms. Without this, a quote
// inside a literal such as ”'it's”' would be read as the end of the string and
// the conditional's real delimiters would be missed.
func skipCELLiteral(s string, i int) (int, bool) {
	type literal struct {
		open  string
		close string
		raw   bool
	}
	candidates := []literal{
		{open: `r'''`, close: `'''`, raw: true},
		{open: `R'''`, close: `'''`, raw: true},
		{open: `r"""`, close: `"""`, raw: true},
		{open: `R"""`, close: `"""`, raw: true},
		{open: `'''`, close: `'''`},
		{open: `"""`, close: `"""`},
		{open: `r'`, close: `'`, raw: true},
		{open: `R'`, close: `'`, raw: true},
		{open: `r"`, close: `"`, raw: true},
		{open: `R"`, close: `"`, raw: true},
		{open: `'`, close: `'`},
		{open: `"`, close: `"`},
		{open: "`", close: "`", raw: true},
	}
	for _, candidate := range candidates {
		if !strings.HasPrefix(s[i:], candidate.open) {
			continue
		}
		// A raw prefix only starts a literal at a token boundary, so an
		// identifier ending in 'r' is not one.
		if candidate.open[0] == 'r' || candidate.open[0] == 'R' {
			if i > 0 && isIdentifierByte(s[i-1]) {
				continue
			}
		}
		escaped := false
		for j := i + len(candidate.open); j < len(s); j++ {
			if !candidate.raw && escaped {
				escaped = false
				continue
			}
			if strings.HasPrefix(s[j:], candidate.close) {
				return j + len(candidate.close), true
			}
			if !candidate.raw && s[j] == '\\' {
				escaped = true
			}
		}
		return 0, false
	}
	return 0, false
}

// isIdentifierByte reports whether b can be part of a CEL identifier segment.
func isIdentifierByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// followsDot reports whether the character before i, ignoring whitespace, is a
// dot, which makes a '?' an optional-traversal marker rather than a ternary.
func followsDot(s string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch s[j] {
		case ' ', '\t', '\r', '\n':
			continue
		case '.':
			return true
		default:
			return false
		}
	}
	return false
}
