package context

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	jmespath "github.com/jmespath/go-jmespath"
)

//Query the JSON context with JMESPATH search path
func (ctx *Context) Query(query string) (interface{}, error) {
	var emptyResult interface{}
	// check for white-listed variables
	if ctx.isWhiteListed(query) {
		return emptyResult, fmt.Errorf("variable %s cannot be used", query)
	}

	// compile the query
	queryPath, err := jmespath.Compile(query)
	if err != nil {
		glog.V(4).Infof("incorrect query %s: %v", query, err)
		return emptyResult, fmt.Errorf("incorrect query %s: %v", query, err)
	}
	// search
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	var data interface{}
	if err := json.Unmarshal(ctx.jsonRaw, &data); err != nil {
		glog.V(4).Infof("failed to unmarshall context: %v", err)
		return emptyResult, fmt.Errorf("failed to unmarshall context: %v", err)
	}

	result, err := queryPath.Search(data)
	if err != nil {
		glog.V(4).Infof("failed to search query %s: %v", query, err)
		return emptyResult, fmt.Errorf("failed to search query %s: %v", query, err)
	}
	return result, nil
}

func (ctx *Context) isWhiteListed(variable string) bool {
	for _, wVar := range ctx.whiteListVars {
		if wVar == variable {
			return true
		}
	}
	return false
}
