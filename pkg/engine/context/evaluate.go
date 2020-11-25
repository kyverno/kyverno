package context

import (
	"encoding/json"
	"fmt"
	"strings"

	jmespath "github.com/jmespath/go-jmespath"
)

//Query the JSON context with JMESPATH search path
func (ctx *Context) Query(query string) (interface{}, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("invalid query (nil)")
	}

	var emptyResult interface{}
	// check for white-listed variables
	if !ctx.isBuiltInVariable(query) {
		return emptyResult, fmt.Errorf("variable %s cannot be used", query)
	}

	// compile the query
	queryPath, err := jmespath.Compile(query)
	if err != nil {
		ctx.log.Error(err, "incorrect query", "query", query)
		return emptyResult, fmt.Errorf("incorrect query %s: %v", query, err)
	}
	// search
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	var data interface{}
	if err := json.Unmarshal(ctx.jsonRaw, &data); err != nil {
		ctx.log.Error(err, "failed to unmarshal context")
		return emptyResult, fmt.Errorf("failed to unmarshall context: %v", err)
	}

	result, err := queryPath.Search(data)
	if err != nil {
		ctx.log.Error(err, "failed to search query", "query", query)
		return emptyResult, fmt.Errorf("failed to search query %s: %v", query, err)
	}
	return result, nil
}

func (ctx *Context) isBuiltInVariable(variable string) bool {
	if len(ctx.builtInVars) == 0 {
		return true
	}
	for _, wVar := range ctx.builtInVars {
		if strings.HasPrefix(variable, wVar) {
			return true
		}
	}
	return false
}
