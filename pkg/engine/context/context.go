package context

import (
	"encoding/json"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

//Interface to manage context operations
type Interface interface {
	//AddJSON  merges the json with context
	AddJSON(dataRaw []byte) error
	//AddResource merges resource json under request.object
	AddResource(dataRaw []byte) error
	//AddUserInfo merges userInfo json under kyverno.userInfo
	AddUserInfo(userInfo kyverno.UserInfo) error
	//AddSA merges serrviceaccount
	AddSA(userName string) error
	EvalInterface
}

//EvalInterface ... to evaluate
type EvalInterface interface {
	Query(query string) (interface{}, error)
}

//Context stores the data resources as JSON
type Context struct {
	mu sync.RWMutex
	// data    map[string]interface{}
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

//AddResource data at path: request.object
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

//AddUserInfo adds userInfo at path request.userInfo
func (ctx *Context) AddUserInfo(userRequestInfo kyverno.RequestInfo) error {
	modifiedResource := struct {
		Request interface{} `json:"request"`
	}{
		Request: userRequestInfo,
	}

	objRaw, err := json.Marshal(modifiedResource)
	if err != nil {
		glog.V(4).Infof("failed to marshall the updated context data")
		return err
	}
	return ctx.AddJSON(objRaw)
}

//AddSA removes prefix 'system:serviceaccount:' and namespace, then loads only SA name and SA namespace
func (ctx *Context) AddSA(userName string) error {
	saPrefix := "system:serviceaccount:"
	var sa string
	saName := ""
	saNamespace := ""
	if len(userName) <= len(saPrefix) {
		sa = ""
	} else {
		sa = userName[len(saPrefix):]
	}
	// filter namespace
	groups := strings.Split(sa, ":")
	if len(groups) >= 2 {
		glog.V(4).Infof("serviceAccount namespace: %s", groups[0])
		glog.V(4).Infof("serviceAccount name: %s", groups[1])
		saName = groups[1]
		saNamespace = groups[0]
	}

	glog.V(4).Infof("Loading variable serviceAccountName with value: %s", saName)
	saNameObj := struct {
		SA string `json:"serviceAccountName"`
	}{
		SA: saName,
	}
	saNameRaw, err := json.Marshal(saNameObj)
	if err != nil {
		glog.V(4).Infof("failed to marshall the updated context data")
		return err
	}
	if err := ctx.AddJSON(saNameRaw); err != nil {
		return err
	}

	glog.V(4).Infof("Loading variable serviceAccountNamespace with value: %s", saNamespace)
	saNsObj := struct {
		SA string `json:"serviceAccountNamespace"`
	}{
		SA: saNamespace,
	}
	saNsRaw, err := json.Marshal(saNsObj)
	if err != nil {
		glog.V(4).Infof("failed to marshall the updated context data")
		return err
	}
	if err := ctx.AddJSON(saNsRaw); err != nil {
		return err
	}

	return nil
}
