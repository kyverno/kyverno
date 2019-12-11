package context

import (
	"github.com/golang/glog"
	jmespath "github.com/jmespath/go-jmespath"
)

//Query searches for query in the context
func (ctx *Context) Query(query string) (interface{}, error) {
	var emptyResult interface{}
	// compile the query
	queryPath, err := jmespath.Compile(query)
	if err != nil {
		glog.V(4).Infof("incorrect query %s: %v", query, err)
		return emptyResult, err
	}

	// search
	result, err := queryPath.Search(ctx.getData())
	if err != nil {
		glog.V(4).Infof("failed to search query %s: %v", query, err)
		return emptyResult, err
	}
	return result, nil
}
