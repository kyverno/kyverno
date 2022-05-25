package context

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/pkg/errors"
)

// Query the JSON context with JMESPATH search path
func (ctx *context) Query(query string) (interface{}, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("invalid query (nil)")
	}
	// compile the query
	queryPath, err := jmespath.New(query)
	if err != nil {
		logger.Error(err, "incorrect query", "query", query)
		return nil, fmt.Errorf("incorrect query %s: %v", query, err)
	}
	// search
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	var data interface{}
	if err := json.Unmarshal(ctx.jsonRaw, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal context")
	}
	result, err := queryPath.Search(data)
	if err != nil {
		return nil, errors.Wrap(err, "JMESPath query failed")
	}
	return result, nil
}

func (ctx *context) HasChanged(jmespath string) (bool, error) {
	objData, err := ctx.Query("request.object." + jmespath)
	if err != nil {
		return false, errors.Wrap(err, "failed to query request.object")
	}
	if objData == nil {
		return false, fmt.Errorf("request.object.%s not found", jmespath)
	}
	oldObjData, err := ctx.Query("request.oldObject." + jmespath)
	if err != nil {
		return false, errors.Wrap(err, "failed to query request.object")
	}
	if oldObjData == nil {
		return false, fmt.Errorf("request.oldObject.%s not found", jmespath)
	}
	return !reflect.DeepEqual(objData, oldObjData), nil
}
