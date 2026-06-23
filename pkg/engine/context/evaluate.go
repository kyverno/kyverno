package context

import (
	"errors" // Added to support errors.As
	"fmt"
	"strings"

	gojmespath "github.com/kyverno/go-jmespath"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
)

// safeSplitLogicalOr explicitly splits a JMESPath query by '||' while respecting string literals.
func safeSplitLogicalOr(query string) []string {
	var parts []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false

	chars := []rune(query)
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c == '\'' && !inDoubleQuote && !inBacktick {
			inSingleQuote = !inSingleQuote
		} else if c == '"' && !inSingleQuote && !inBacktick {
			inDoubleQuote = !inDoubleQuote
		} else if c == '`' && !inSingleQuote && !inDoubleQuote {
			inBacktick = !inBacktick
		}

		if !inSingleQuote && !inDoubleQuote && !inBacktick && c == '|' && i+1 < len(chars) && chars[i+1] == '|' {
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

// Query the JSON context with JMESPATH search path.
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
				for _, part := range parts {
					partPath, partErr := ctx.jp.Query(part)
					if partErr != nil {
						continue
					}
					partResult, partSearchErr := partPath.Search(ctx.jsonRaw)
					// Short-circuit on the first successful, truthy result
					if partSearchErr == nil && partResult != nil {
						return partResult, nil
					}
				}
			}
		}
		// If it is not a fallback query, return the original error so we don't break existing behaviors
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