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

// jmespathMultiSelectIdentifiers matches a JMESPath expression that starts with a multi-select list
// e.g. "[allPods, static]" or "[allPods, static] []" and captures the comma-separated identifiers.
// Note: This pattern does not support nested brackets (e.g., [[a, b], c]) or quoted identifiers.
// Only simple identifier names are validated; trailing operators (e.g., [] flatten, | pipes) are ignored.
var jmespathMultiSelectIdentifiers = regexp.MustCompile(`^\s*\[\s*([^\[\]]+)\]\s*`)

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
	if ctx.re != nil && ctx.re.MatchString(query) {
		return emptyResult, nil
	}

	if ctx.isVariableDefined(query) {
		return emptyResult, nil
	}

	if submatches := jmespathMultiSelectIdentifiers.FindStringSubmatch(query); len(submatches) == 2 {
		identifiersStr := strings.TrimSpace(submatches[1])
		for _, ident := range strings.Split(identifiersStr, ",") {
			ident = strings.TrimSpace(ident)
			if ident == "" {
				continue
			}
			if ctx.re != nil && ctx.re.MatchString(ident) {
				continue
			}
			if ctx.isVariableDefined(ident) {
				continue
			}
			return emptyResult, InvalidVariableError{
				variable:        ident,
				re:              ctx.re,
				allowedPatterns: ctx.allowedPatterns,
			}
		}
		return emptyResult, nil
	}

	return emptyResult, InvalidVariableError{
		variable:        query,
		re:              ctx.re,
		allowedPatterns: ctx.allowedPatterns,
	}
}

func (ctx *MockContext) QueryOperation() string {
	if op, err := ctx.Query("request.operation"); err != nil {
		if op != nil {
			return op.(string)
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
