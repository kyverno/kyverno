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

		// Handle escape characters so we don't misread escaped quotes
		if c == '\\' && i+1 < len(chars) {
			current.WriteRune(c)
			i++
			current.WriteRune(chars[i])
			continue
		}

		if c == '\'' && !inDoubleQuote && !inBacktick {
			inSingleQuote = !inSingleQuote
		} else if c == '"' && !inSingleQuote && !inBacktick {
			inDoubleQuote = !inDoubleQuote
		} else if c == '`' && !inSingleQuote && !inDoubleQuote {
			inBacktick = !inBacktick
		}

		// Track nesting depth for brackets, braces, and parentheses
		if !inSingleQuote && !inDoubleQuote && !inBacktick {
			if c == '[' || c == '{' || c == '(' {
				depth++
			} else if c == ']' || c == '}' || c == ')' {
				depth--
			}
		}

		// Only split on || if we are at the root depth (depth == 0) and not in quotes
		if !inSingleQuote && !inDoubleQuote && !inBacktick && depth == 0 && c == '|' && i+1 < len(chars) && chars[i+1] == '|' {
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
			i++ // skip the second |
			continue
		}
		current.WriteRune(c)
	}
	parts = append(parts, strings.TrimSpace(current.String()))
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

// Query the JSON context with JMESPATH search path.
// Note: This method implements custom handling for JMESPath logical OR ('||') expressions.
// When a NotFoundError occurs, it explicitly evaluates operands sequentially, treating missing
// keys as null. This prevents the query from failing early and allows fallbacks to evaluate
// correctly according to standard JMESPath truthiness rules.
func (ctx *context) Query(query string) (interface{}, error) {
	if err := ctx.loadDeferred(query); err != nil {
		return nil, err
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("invalid query (nil)")
	}
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
			if len(parts) > 1 {
				var lastResult interface{}
				for _, part := range parts {
					partPath, partErr := ctx.jp.Query(part)
					if partErr != nil {
						// Do not swallow compile errors in the fallback operand
						return nil, fmt.Errorf("incorrect fallback query %s: %w", part, partErr)
					}
					partResult, partSearchErr := partPath.Search(ctx.jsonRaw)

					if partSearchErr != nil {
						var partNotFound gojmespath.NotFoundError
						if errors.As(partSearchErr, &partNotFound) {
							// Missing key, treat as null
							lastResult = nil
						} else {
							// Do not swallow other runtime errors
							return nil, fmt.Errorf("fallback JMESPath query failed: %w", partSearchErr)
						}
					} else {
						lastResult = partResult
						// Short-circuit on the first successful, truthy result
						if isTruthy(partResult) {
							return partResult, nil
						}
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
