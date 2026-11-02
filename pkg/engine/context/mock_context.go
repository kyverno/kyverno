package context

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/ext/wildcard"
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

	// Parse the query into an AST once. This both validates the JMESPath syntax
	// (replacing the separate compile-only check that used to live here) and, for
	// multi-select list expressions below, gives us a proper tree of field/literal/
	// operator nodes instead of raw text.
	ast, err := gojmespath.NewParser().Parse(query)
	if err != nil {
		return emptyResult, fmt.Errorf("invalid JMESPath query %s: %v", query, err)
	}

	if strings.HasPrefix(query, "[") {
		// Multi-select list expressions (e.g. "[a, b]", "[a, b] | c", "[a, b] || c")
		// get full structural validation: every identifier referenced anywhere in the
		// expression -- across elements, logical or/and expressions, function-call
		// arguments, piped or or-ed suffixes, etc. -- is checked against the
		// allow-list, so an allowed identifier appearing elsewhere in the fragment
		// can't mask a forbidden one. This replaces the earlier hand-rolled,
		// depth-tracking string splitter, which could only ever reason about raw
		// text and kept needing new special cases for every JMESPath construct it
		// hadn't considered -- most recently: an allowed identifier masking a
		// forbidden one via whole-fragment regex matching, JMESPath literals being
		// mistaken for context-variable identifiers, and "||" being mistaken for a
		// "|" pipe when trimming suffixes.
		if err := ctx.validateNode(ast, true); err != nil {
			return emptyResult, err
		}
		return emptyResult, nil
	}

	// Outside of multi-select lists, preserve the historical whole-fragment check:
	// the allow-list regex configured by callers (see e.g. ReservedKeys) also encodes
	// "this looks like a function call" via its `([a-z_0-9]+\()[^{}]` alternative,
	// which intentionally exempts function-call arguments (and other
	// sub-expressions, e.g. a piped "| merge(@, {k: v})") from per-identifier
	// validation; plenty of policies rely on that leniency for custom/builtin
	// JMESPath functions whose argument names aren't context variables at all.
	//
	// strip escaped quotes from JMESPath variables with dashes e.g. {{ "my-map.data".key }}
	strippedQuery := strings.ReplaceAll(query, "\"", "")
	if err := ctx.validateVariable(strippedQuery); err != nil {
		return emptyResult, err
	}

	return emptyResult, nil
}

// validateNode walks a parsed JMESPath AST node and validates every identifier it
// references against the allow-list regex (ctx.re) and the registered wildcard variable
// patterns, so that a forbidden identifier can't hide behind an allowed one anywhere in
// the expression -- inside multi-select lists, logical or/and expressions, pipes,
// function-call arguments, filter projections, hash values, etc.
//
// Nodes that reduce to a single identifier/field path (e.g. "request.object", "allPods",
// "@") are validated as one unit via pathString, exactly like the historical
// whole-fragment check -- this preserves allow-list patterns that depend on the full
// dotted path (e.g. the regex alternative "request\." or a wildcard pattern like
// "my-map.data.*"). Anything else is decomposed and every operand is validated
// independently: JMESPath literals (quoted strings, backtick JSON literals, numbers, ...)
// are values, not context-variable references, and are skipped entirely; a function's
// name and a hash's keys are likewise not variables, so only their children (arguments /
// values) are visited.
//
// isTop distinguishes the top-level call (the whole query) from recursive calls: a
// no-op sub-path (e.g. the unused projection target in "foo[?bar==`1`]") contributes
// nothing and is skipped, but a query that is -- in its entirety -- a no-op with no
// identifier at all (e.g. a bare "[]" or "[*]") is still rejected by default, same as
// before.
func (ctx *MockContext) validateNode(node gojmespath.ASTNode, isTop bool) error {
	if path, ok := pathString(node); ok {
		if path == "" && !isTop {
			return nil
		}
		return ctx.validateVariable(path)
	}

	if node.NodeType == gojmespath.ASTLiteral {
		return nil
	}

	for _, child := range node.Children {
		if err := ctx.validateNode(child, false); err != nil {
			return err
		}
	}

	return nil
}

// validateVariable validates a single, fully-resolved JMESPath identifier path (e.g.
// "request.object" or "allPods") against the allow-list regex and the registered
// wildcard variable patterns.
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

// pathString attempts to render node as a single dotted identifier/field path (e.g.
// "request.object", "allPods", "@") by walking the pure path-building node types: field
// access, dotted subexpressions, indexing, flattening, and projections, plus the
// current-node token. ok is false if node -- or one of its descendants reached while
// walking it -- is not purely a path, e.g. it embeds a literal, a function call, or a
// logical/pipe operator; the caller should then fall back to validating node's children
// independently rather than treating it as one reference.
func pathString(node gojmespath.ASTNode) (string, bool) {
	switch node.NodeType {
	case gojmespath.ASTField:
		name, ok := node.Value.(string)
		return name, ok
	case gojmespath.ASTCurrentNode:
		return "@", true
	case gojmespath.ASTIdentity, gojmespath.ASTIndex, gojmespath.ASTSlice:
		// These don't contribute an identifier of their own -- they continue, or
		// index/slice into, whatever path precedes them.
		return "", true
	case gojmespath.ASTSubexpression, gojmespath.ASTIndexExpression,
		gojmespath.ASTProjection, gojmespath.ASTFlatten, gojmespath.ASTValueProjection:
		var parts []string
		for _, child := range node.Children {
			part, ok := pathString(child)
			if !ok {
				return "", false
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "."), true
	default:
		return "", false
	}
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
