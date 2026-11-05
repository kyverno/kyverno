package context

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	gojmespath "github.com/kyverno/go-jmespath"
	enginejmespath "github.com/kyverno/kyverno/pkg/engine/jmespath"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
)

// safeSplitLogicalOr splits a query on top-level || operators, unless it
// contains a top-level pipe. JMESPath evaluates || more tightly than |, so
// splitting an expression with a top-level pipe could change its meaning.
// Nested operators are left intact to preserve JMESPath's relative projection
// scope.
func safeSplitLogicalOr(query string) []string {
	if !strings.Contains(query, "||") {
		return []string{query}
	}

	chars := []rune(query)
	var parts []string
	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	depth := 0

	for i := 0; i < len(chars); i++ {
		c := chars[i]

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

		if depth == 0 && !inSingleQuote && !inDoubleQuote && !inBacktick && c == '|' {
			if i+1 < len(chars) && chars[i+1] == '|' {
				operand := strings.TrimSpace(current.String())
				if operand == "" {
					// Keep malformed operators intact so the JMESPath compiler
					// reports the syntax error for the whole query.
					return []string{query}
				}
				parts = append(parts, operand)
				current.Reset()
				i++
				continue
			}

			// A top-level pipe binds more loosely than ||. Keep the full
			// expression intact so splitting cannot change its precedence.
			return []string{query}
		}

		current.WriteRune(c)
	}

	if len(parts) == 0 {
		return []string{query}
	}

	last := strings.TrimSpace(current.String())
	if last == "" {
		// Preserve malformed trailing operators so the JMESPath compiler
		// reports the syntax error instead of evaluating a truncated chain.
		return []string{query}
	}

	parts = append(parts, last)
	return parts
}

// isTruthy evaluates JMESPath truthiness, including concrete slice and map
// types returned by custom JMESPath functions.
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

	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.String:
		return value.Len() > 0
	case reflect.Bool:
		return value.Bool()
	case reflect.Array, reflect.Slice, reflect.Map:
		return value.Len() > 0
	case reflect.Pointer, reflect.Interface:
		return !value.IsNil()
	default:
		return true
	}
}

// Query the JSON context with a JMESPath search path.
// Composite query results may reference live context data and must be
// treated as read-only.
// Context writes must go through the context mutation APIs.
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

// evaluateQuery compiles and evaluates a JMESPath query. Eligible top-level
// logical-OR chains are evaluated operand by operand so Kyverno's truthiness
// rules also apply to concrete values returned by custom functions. Only
// NotFoundError triggers fallback; other errors propagate. If every operand
// is missing, the returned error wraps the final operand's NotFoundError.
func (ctx *context) evaluateQuery(query string) (interface{}, error) {
	parts := safeSplitLogicalOr(query)
	if len(parts) > 1 {
		compiledParts := make([]enginejmespath.Query, len(parts))
		for i, part := range parts {
			queryPath, err := ctx.jp.Query(part)
			if err != nil {
				logger.Error(err, "incorrect query", "query", query)
				return nil, fmt.Errorf("incorrect query %s: %w", query, err)
			}
			compiledParts[i] = queryPath
		}

		var lastResult interface{}
		var lastNotFound error
		hadSuccessfulOperand := false

		for i, part := range parts {
			partResult, partErr := compiledParts[i].Search(ctx.jsonRaw)
			if partErr != nil {
				var partNotFound gojmespath.NotFoundError
				if errors.As(partErr, &partNotFound) {
					lastResult = nil
					lastNotFound = partErr
					continue
				}

				return nil, fmt.Errorf("fallback JMESPath query %q failed: %w", part, partErr)
			}

			hadSuccessfulOperand = true
			lastResult = partResult
			lastNotFound = nil
			if isTruthy(partResult) {
				return partResult, nil
			}
		}

		if lastNotFound != nil && !hadSuccessfulOperand {
			return nil, fmt.Errorf("JMESPath query failed: %w", lastNotFound)
		}
		return lastResult, nil
	}

	queryPath, err := ctx.jp.Query(query)
	if err != nil {
		logger.Error(err, "incorrect query", "query", query)
		return nil, fmt.Errorf("incorrect query %s: %w", query, err)
	}

	result, err := queryPath.Search(ctx.jsonRaw)
	if err != nil {
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