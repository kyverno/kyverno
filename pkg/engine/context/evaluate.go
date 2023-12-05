package context

import (
	"fmt"
	"strings"

	datautils "github.com/kyverno/kyverno/pkg/utils/data"
)

// Query the JSON context with JMESPATH search path
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
