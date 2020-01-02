package context

import (
	"encoding/json"
	"sync"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	authenticationv1 "k8s.io/api/authentication/v1"
)

//Interface ... normal functions
type Interface interface {
	// merges the json with context
	AddJSON(dataRaw []byte) error
	// merges resource json under request.object
	AddResource(dataRaw []byte) error
	// merges userInfo json under request.userInfo
	AddUserInfo(userInfo authenticationv1.UserInfo) error
	EvalInterface
}

//EvalInterface ... to evaluate
type EvalInterface interface {
	Query(query string) (interface{}, error)
}

//Context stores the data resources as JSON
type Context struct {
	mu      sync.RWMutex
	jsonRaw []byte
}

//NewContext returns a new context
func NewContext() *Context {
	ctx := Context{
		// data:    map[string]interface{}{},
		jsonRaw: []byte(`{}`), // empty json struct
	}
	return &ctx
}

// AddJSON merges json data
func (ctx *Context) AddJSON(dataRaw []byte) error {
	var err error
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	// merge json
	ctx.jsonRaw, err = jsonpatch.MergePatch(ctx.jsonRaw, dataRaw)
	if err != nil {
		glog.V(4).Infof("failed to merge JSON data: %v", err)
		return err
	}
	return nil
}

//AddResource adds data at path: request.object
func (ctx *Context) AddResource(dataRaw []byte) error {

	// unmarshall the resource struct
	var data interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		glog.V(4).Infof("failed to unmarshall the context data: %v", err)
		return err
	}

	modifiedResource := struct {
		Request interface{} `json:"request"`
	}{
		Request: struct {
			Object interface{} `json:"object"`
		}{
			Object: data,
		},
	}

	objRaw, err := json.Marshal(modifiedResource)
	if err != nil {
		glog.V(4).Infof("failed to marshall the updated context data")
		return err
	}
	return ctx.AddJSON(objRaw)
}

//AddUserInfo adds data at path: request.userInfo
func (ctx *Context) AddUserInfo(userInfo authenticationv1.UserInfo) error {
	modifiedResource := struct {
		Request interface{} `json:"request"`
	}{
		Request: struct {
			UserInfo interface{} `json:"userInfo"`
		}{
			UserInfo: userInfo,
		},
	}

	objRaw, err := json.Marshal(modifiedResource)
	if err != nil {
		glog.V(4).Infof("failed to marshall the updated context data")
		return err
	}
	return ctx.AddJSON(objRaw)
}
