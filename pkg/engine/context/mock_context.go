package context

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

// MockContext is used for testing and validation of variables
type MockContext struct {
	mutex           sync.RWMutex
	re              *regexp.Regexp
	allowedPatterns []string
}

// NewMockContext creates a new MockContext that allows variables matching the supplied list of wildcard patterns
func NewMockContext(re *regexp.Regexp, vars ...string) *MockContext {
	return &MockContext{re: re, allowedPatterns: vars}
}

// AddVariable adds given wildcardPattern to the allowed variable patterns
func (ctx *MockContext) AddVariable(wildcardPattern string) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	builtInVarsCopy := ctx.allowedPatterns
	ctx.allowedPatterns = append(builtInVarsCopy, wildcardPattern)
}

// Query the JSON context with JMESPATH search path
func (ctx *MockContext) Query(query string) (interface{}, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("invalid query (nil)")
	}

	var emptyResult interface{}

	// compile the query
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	if _, err := jp.Query(query); err != nil {
		return emptyResult, fmt.Errorf("invalid JMESPath query %s: %v", query, err)
	}

	// strip escaped quotes from JMESPath variables with dashes e.g. {{ \"my-map.data\".key }}
	query = strings.Replace(query, "\"", "", -1)

	if err := ctx.validateExpression(query); err != nil {
		return emptyResult, err
	}

	return emptyResult, nil
}

// validateExpression validates that every context variable referenced by a JMESPath
// expression is allowed, i.e. it matches either the configured allow-list regex (ctx.re)
// or one of the registered wildcard variable patterns.
//
// Multi-select list expressions (e.g. "[a, b]") are handled before falling back to the
// whole-expression regex/pattern check below, so that a leading allowed element (e.g.
// "request.object") can't cause a disallowed trailing element (e.g. "typo") to be skipped.
func (ctx *MockContext) validateExpression(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}

	if strings.HasPrefix(expr, "[") {
		return ctx.validateMultiSelect(expr)
	}

	return ctx.validateVariable(expr)
}

// validateVariable validates a single, non multi-select JMESPath fragment against the
// allow-list regex and the registered wildcard variable patterns.
func (ctx *MockContext) validateVariable(variable string) error {
	if ctx.re != nil && ctx.re.MatchString(variable) {
		return nil
	}

	if ctx.isVariableDefined(variable) {
		return nil
	}

	return InvalidVariableError{
		variable:        variable,
		re:              ctx.re,
		allowedPatterns: ctx.allowedPatterns,
	}
}

// validateMultiSelect validates a JMESPath expression that starts with a multi-select list,
// e.g. "[a, b]" or "[a, b] | c". It splits the bracketed content into its top-level,
// comma-separated elements -- tracking bracket/brace/paren nesting so that nested
// multi-select lists (e.g. "[[a, b], c]") and elements that carry their own brackets (e.g.
// "a[]") are split correctly instead of being excluded by a bracket-free pattern -- then
// validates every element individually (recursing for nested multi-selects), and finally
// validates whatever trails the closing bracket (e.g. a piped expression such as "| c"),
// so no referenced variable is ignored.
func (ctx *MockContext) validateMultiSelect(expr string) error {
	elements, rest, ok := splitMultiSelect(expr)
	if !ok {
		// Not a well-formed multi-select list (e.g. unbalanced brackets); fall back to
		// validating the raw fragment rather than silently accepting it.
		return ctx.validateVariable(expr)
	}

	for _, element := range elements {
		element = strings.TrimSpace(element)
		if element == "" {
			return InvalidVariableError{
				variable:        expr,
				re:              ctx.re,
				allowedPatterns: ctx.allowedPatterns,
			}
		}

		if err := ctx.validateExpression(element); err != nil {
			return err
		}
	}

	return ctx.validateMultiSelectSuffix(rest)
}

// validateMultiSelectSuffix validates whatever follows the closing bracket of a
// multi-select list. Bare flatten/projection operators ("[]", "[*]") apply to the
// preceding result and don't reference a new variable, so they're skipped; anything
// else -- most commonly a piped expression, e.g. "| c" -- is validated like any other
// JMESPath fragment.
func (ctx *MockContext) validateMultiSelectSuffix(rest string) error {
	rest = stripBareProjectionSuffix(rest)
	if rest == "" {
		return nil
	}

	rest = strings.TrimPrefix(rest, "|")
	return ctx.validateExpression(rest)
}

// stripBareProjectionSuffix repeatedly strips leading bare flatten ("[]") or projection
// ("[*]") operators and surrounding whitespace, returning whatever (if anything) remains.
func stripBareProjectionSuffix(s string) string {
	for {
		trimmed := strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(trimmed, "[]"):
			s = trimmed[len("[]"):]
		case strings.HasPrefix(trimmed, "[*]"):
			s = trimmed[len("[*]"):]
		default:
			return trimmed
		}
	}
}

// splitMultiSelect splits a JMESPath expression that begins with a multi-select list (e.g.
// "[a, b] | c") into its top-level, comma-separated elements ("a", "b") and the remainder
// of the expression following the matching closing bracket (" | c"). Bracket/brace/paren
// nesting is tracked so that elements containing their own brackets (e.g. "a[]") or nested
// multi-select lists (e.g. "[a, b]") aren't split incorrectly. ok is false if expr doesn't
// start with "[" or its brackets are unbalanced.
func splitMultiSelect(expr string) (elements []string, rest string, ok bool) {
	trimmed := strings.TrimSpace(expr)
	if !strings.HasPrefix(trimmed, "[") {
		return nil, "", false
	}

	depth := 0
	start := -1
	for i, r := range trimmed {
		switch r {
		case '[', '(', '{':
			depth++
			if depth == 1 {
				start = i + 1
			}
		case ']', ')', '}':
			depth--
			if depth < 0 {
				return nil, "", false
			}
			if depth == 0 {
				elements = append(elements, trimmed[start:i])
				return elements, trimmed[i+1:], true
			}
		case ',':
			if depth == 1 {
				elements = append(elements, trimmed[start:i])
				start = i + 1
			}
		}
	}

	// unbalanced brackets
	return nil, "", false
}

func (ctx *MockContext) QueryOperation() string {
	if op, err := ctx.Query("request.operation"); err != nil {
		if opStr, ok := op.(string); ok {
			return opStr
		}
	}

	return ""
}

func (ctx *MockContext) isVariableDefined(variable string) bool {
	for _, pattern := range ctx.getVariables() {
		if wildcard.Match(pattern, variable) {
			return true
		}
	}

	return false
}

func (ctx *MockContext) getVariables() []string {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	vars := ctx.allowedPatterns
	return vars
}

// InvalidVariableError represents error for non-white-listed variables
type InvalidVariableError struct {
	variable        string
	re              *regexp.Regexp
	allowedPatterns []string
}

func (i InvalidVariableError) Error() string {
	if i.re == nil {
		return fmt.Sprintf("variable %s must match patterns %v", i.variable, i.allowedPatterns)
	}

	return fmt.Sprintf("variable %s must match regex \"%s\" or patterns %v", i.variable, i.re.String(), i.allowedPatterns)
}

func (ctx *MockContext) HasChanged(_ string) (bool, error) {
	return false, nil
}
