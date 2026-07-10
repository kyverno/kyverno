package context

import (
	"errors"
	"fmt"
	"strings"

	gojmespath "github.com/kyverno/go-jmespath"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
)

// safeSplitLogicalOr explicitly splits a JMESPath query by '||'
// while respecting string literals, escapes, and nesting depth.
func safeSplitLogicalOr(query string) []string {
	var parts []string
	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	depth := 0

	chars := []rune(query)
	for i := 0; i < len(chars); i++ {
		c := chars[i]

		// Handle escape characters inside string/backtick literals so we don't misread escaped delimiters
		if (inSingleQuote || inDoubleQuote || inBacktick) && c == '\\' && i+1 < len(chars) {
			current.WriteRune(c)
			i++
			current.WriteRune(chars[i])
			continue
		}

		switch c {
		case '\'':
			if !inDoubleQuote && !inBacktick {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote && !inBacktick {
				inDoubleQuote = !inDoubleQuote
			}
		case '`':
			if !inSingleQuote && !inDoubleQuote {
				inBacktick = !inBacktick
			}
		case '(', '[', '{':
			if !inSingleQuote && !inDoubleQuote && !inBacktick {
				depth++
			}
		case ')', ']', '}':
			if !inSingleQuote && !inDoubleQuote && !inBacktick {
				depth--
			}
		}

		// Split on '||' only if we are at depth 0 and not inside any quotes/backticks
		if depth == 0 && !inSingleQuote && !inDoubleQuote && !inBacktick {
			if c == '|' && i+1 < len(chars) && chars[i+1] == '|' {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
				i++ // Skip the second '|'
				continue
			}
		}

		current.WriteRune(c)
	}

	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}

	return parts
}

// isTruthy evaluates standard JMESPath truthiness rules
func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return len(val) > 0
	case []interface{}:
		return len(val) > 0
	case map[string]interface{}:
		return len(val) > 0
	}
	return true
}

// Query the JSON context with a JMESPath search path.
// Note: If the query contains a top-level logical OR ('||') and evaluation fails with a NotFoundError,
// operands are evaluated left-to-right and missing keys are treated as null to allow fallbacks
// to run according to standard JMESPath truthiness rules. Nested '||' expressions inside an
// operand (e.g. within parentheses) are evaluated recursively using the same fallback rules.
func (ctx *context) Query(query string) (interface{}, error) {
	if err := ctx.loadDeferred(query); err != nil {
		return nil, err
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("invalid query (nil)")
	}

	return ctx.evaluateQuery(query)
}

// evaluateQuery compiles and searches a JMESPath query, applying logical OR
// fallback handling on NotFoundError. It is safe to call recursively for
// nested fallback operands because it never triggers loadDeferred — that
// happens exactly once, in Query.
func (ctx *context) evaluateQuery(query string) (interface{}, error) {
	// compile the query
	queryPath, err := ctx.jp.Query(query)
	if err != nil {
		logger.Error(err, "incorrect query", "query", query)
		return nil, fmt.Errorf("incorrect query %s: %v", query, err)
	}

	// search
	result, err := queryPath.Search(ctx.jsonRaw)
	if err != nil {
		var notFoundErr gojmespath.NotFoundError
		if errors.As(err, &notFoundErr) && strings.Contains(query, "||") {
			// Explicitly evaluate the || chain to allow logical fallbacks
			parts := safeSplitLogicalOr(query)
			trimmed := strings.TrimSpace(query)
			for len(parts) == 1 && strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
				trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
				parts = safeSplitLogicalOr(trimmed)
			}
			if len(parts) > 1 {
				var lastResult interface{}
				for _, part := range parts {
					// Recursively evaluate each operand using the same fallback
					// rules. This ensures a nested '||' inside an operand (e.g.
					// "a || (b || 'x')") is not treated as a single opaque
					// expression that returns nil on NotFoundError, but is itself
					// resolved through its own fallback chain.
					partResult, partErr := ctx.evaluateQuery(part)
					if partErr != nil {
						var partNotFound gojmespath.NotFoundError
						if errors.As(partErr, &partNotFound) {
							// Missing key, treat as null
							lastResult = nil
							continue
						}
						// Do not swallow compile errors or other runtime errors
						return nil, fmt.Errorf("fallback JMESPath query %q failed: %w", part, partErr)
					}
					lastResult = partResult
					// Short-circuit on the first successful, truthy result
					if isTruthy(partResult) {
						return partResult, nil
					}
				}
				// If all operands evaluate to a falsey value (or are missing), yield the last operand
				return lastResult, nil
			}
		}
		return nil, fmt.Errorf("JMESPath query failed: %w", err)
	}
	return result, nil
}

func (ctx *context) loadDeferred(query string) error {
	level := len(ctx.jsonRawCheckpoints)
	return ctx.deferred.LoadMatching(query, level)
}

func (ctx *context) HasChanged(jmespath string) (bool, error) {
	objData, err := ctx.Query("request.object." + jmespath)
	if err != nil {
		return false, fmt.Errorf("failed to query request.object: %w", err)
	}
	if objData == nil {
		return false, fmt.Errorf("request.object.%s not found", jmespath)
	}
	oldObjData, err := ctx.Query("request.oldObject." + jmespath)
	if err != nil {
		return false, fmt.Errorf("failed to query request.object: %w", err)
	}
	if oldObjData == nil {
		return false, fmt.Errorf("request.oldObject.%s not found", jmespath)
	}
	return !datautils.DeepEqual(objData, oldObjData), nil
}