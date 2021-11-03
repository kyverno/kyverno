package context

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	jmespath "github.com/kyverno/kyverno/pkg/engine/jmespath"
)

//Query the JSON context with JMESPATH search path
func (ctx *Context) Query(query string) (interface{}, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("invalid query (nil)")
	}

	var emptyResult interface{}

	// compile the query
	queryPath, err := jmespath.New(query)
	if err != nil {
		ctx.log.Error(err, "incorrect query", "query", query)
		return emptyResult, fmt.Errorf("incorrect query %s: %v", query, err)
	}

	// search
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	var data interface{}
	if err := json.Unmarshal(ctx.jsonRaw, &data); err != nil {
		ctx.log.Error(err, "failed to unmarshal context")
		return emptyResult, fmt.Errorf("failed to unmarshal context: %v", err)
	}

	result, err := queryPath.Search(data)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "Unknown key") {
			ctx.log.Error(err, "JMESPath search failed", "query", query)
		}

		return emptyResult, err
	}

	return result, nil
}

func (ctx *Context) HasChanged(jmespath string) (bool, error) {
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

	if reflect.DeepEqual(objData, oldObjData) {
		return false, nil
	}

	return true, nil
}
