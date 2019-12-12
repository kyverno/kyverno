package context

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/golang/glog"
)

//Interface ... normal functions
type Interface interface {
	Add(key string, data []byte) error
	Remove(key string) error
	EvalInterface
}

//EvalInterface ... to evaluate
type EvalInterface interface {
	Query(query string) (interface{}, error)
}

//Context stores the data resources as JSON
type Context struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

//NewContext returns a new context
func NewContext() *Context {
	ctx := Context{
		data: map[string]interface{}{},
	}
	return &ctx
}

//Add adds resource with the key
// we always overwrite the resoruce if already present
func (ctx *Context) Add(key string, resource []byte) error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	// insert/update
	// umarshall before adding
	var data interface{}
	if err := json.Unmarshal(resource, &data); err != nil {
		glog.V(4).Infof("failed to unmarshall resource in context: %v", err)
		fmt.Println(err)
		return err
	}
	ctx.data[key] = data
	return nil
}

//Remove removes resource with given key
func (ctx *Context) Remove(key string) error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	_, ok := ctx.data[key]
	if ok {
		delete(ctx.data, key)
		return nil
	}
	return fmt.Errorf("no resource with key %s", key)
}

func (ctx *Context) getData() interface{} {
	return ctx.data
}
