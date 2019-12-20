package context

import (
	"encoding/json"

	"github.com/golang/glog"
	"github.com/jmespath/go-jmespath"
)

// //Query searches for query in the context
// func (ctx *Context) Query(query string) (interface{}, error) {
// 	var emptyResult interface{}
// 	// compile the query
// 	queryPath, err := jmespath.Compile(query)
// 	if err != nil {
// 		glog.V(4).Infof("incorrect query %s: %v", query, err)
// 		return emptyResult, err
// 	}

// 	// search
// 	result, err := queryPath.Search(ctx.getData())
// 	if err != nil {
// 		glog.V(4).Infof("failed to search query %s: %v", query, err)
// 		return emptyResult, err
// 	}
// 	return result, nil
// }
//Query ...
func (ctx *Context) Query(query string) (interface{}, error) {
	var emptyResult interface{}
	// compile the query
	queryPath, err := jmespath.Compile(query)
	if err != nil {
		glog.V(4).Infof("incorrect query %s: %v", query, err)
		return emptyResult, err
	}
	// search
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	var data interface{}
	if err := json.Unmarshal(ctx.jsonRaw, &data); err != nil {
		glog.V(4).Infof("failed to unmarshall context")
		return emptyResult, err
	}

	result, err := queryPath.Search(data)
	if err != nil {
		glog.V(4).Infof("failed to search query %s: %v", query, err)
		return emptyResult, err
	}
	return result, nil
}
