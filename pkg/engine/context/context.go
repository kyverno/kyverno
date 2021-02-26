package context

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//Interface to manage context operations
type Interface interface {

	// AddJSON  merges the json with context
	AddJSON(dataRaw []byte) error

	// AddResource merges resource json under request.object
	AddResource(dataRaw []byte) error

	// AddUserInfo merges userInfo json under kyverno.userInfo
	AddUserInfo(userInfo kyverno.UserInfo) error

	// AddServiceAccount merges ServiceAccount types
	AddServiceAccount(userName string) error

	// AddNamespace merges resource json under request.namespace
	AddNamespace(namespace string) error

	EvalInterface
}

//EvalInterface ... to evaluate
type EvalInterface interface {
	Query(query string) (interface{}, error)
}

//Context stores the data resources as JSON
type Context struct {
	mutex             sync.RWMutex
	jsonRaw           []byte
	jsonRawCheckpoint []byte
	builtInVars       []string
	log               logr.Logger
}

//NewContext returns a new context
// builtInVars is the list of known variables (e.g. serviceAccountName)
func NewContext(builtInVars ...string) *Context {
	ctx := Context{
		jsonRaw:     []byte(`{}`), // empty json struct
		builtInVars: builtInVars,
		log:         log.Log.WithName("context"),
	}

	return &ctx
}

// InvalidVariableErr represents error for non-white-listed variables
type InvalidVariableErr struct {
	variable  string
	whiteList []string
}

func (i InvalidVariableErr) Error() string {
	return fmt.Sprintf("variable %s cannot be used, allowed variables: %v", i.variable, i.whiteList)
}

// AddJSON merges json data
func (ctx *Context) AddJSON(dataRaw []byte) error {
	var err error
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	// merge json
	ctx.jsonRaw, err = jsonpatch.MergePatch(ctx.jsonRaw, dataRaw)
	if err != nil {
		ctx.log.Error(err, "failed to merge JSON data")
		return err
	}
	return nil
}

// AddRequest adds an admission request to context
func (ctx *Context) AddRequest(request *v1beta1.AdmissionRequest) error {
	modifiedResource := struct {
		Request interface{} `json:"request"`
	}{
		Request: request,
	}

	objRaw, err := json.Marshal(modifiedResource)
	if err != nil {
		ctx.log.Error(err, "failed to marshal the request")
		return err
	}
	return ctx.AddJSON(objRaw)
}

//AddResource data at path: request.object
func (ctx *Context) AddResource(dataRaw []byte) error {

	// unmarshal the resource struct
	var data interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		ctx.log.Error(err, "failed to unmarshal the resource")
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
		ctx.log.Error(err, "failed to marshal the resource")
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
		ctx.log.Error(err, "failed to marshal the UserInfo")
		return err
	}
	return ctx.AddJSON(objRaw)
}

//AddServiceAccount removes prefix 'system:serviceaccount:' and namespace, then loads only SA name and SA namespace
func (ctx *Context) AddServiceAccount(userName string) error {
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
		saName = groups[1]
		saNamespace = groups[0]
	}

	saNameObj := struct {
		SA string `json:"serviceAccountName"`
	}{
		SA: saName,
	}
	saNameRaw, err := json.Marshal(saNameObj)
	if err != nil {
		ctx.log.Error(err, "failed to marshal the SA")
		return err
	}
	if err := ctx.AddJSON(saNameRaw); err != nil {
		return err
	}

	saNsObj := struct {
		SA string `json:"serviceAccountNamespace"`
	}{
		SA: saNamespace,
	}
	saNsRaw, err := json.Marshal(saNsObj)
	if err != nil {
		ctx.log.Error(err, "failed to marshal the SA namespace")
		return err
	}
	if err := ctx.AddJSON(saNsRaw); err != nil {
		return err
	}

	return nil
}

// AddNamespace merges resource json under request.namespace
func (ctx *Context) AddNamespace(namespace string) error {
	modifiedResource := struct {
		Request interface{} `json:"request"`
	}{
		Request: struct {
			Namespace string `json:"namespace"`
		}{
			Namespace: namespace,
		},
	}

	objRaw, err := json.Marshal(modifiedResource)
	if err != nil {
		ctx.log.Error(err, "failed to marshal the resource")
		return err
	}

	return ctx.AddJSON(objRaw)
}

// Checkpoint creates a copy of the internal state.
// Prior checkpoints will be overridden.
func (ctx *Context) Checkpoint() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	ctx.jsonRawCheckpoint = make([]byte, len(ctx.jsonRaw))
	copy(ctx.jsonRawCheckpoint, ctx.jsonRaw)
}

// Restore restores internal state from a prior checkpoint, if one exists.
// If a prior checkpoint does not exist, the state will not be changed.
func (ctx *Context) Restore() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if ctx.jsonRawCheckpoint == nil || len(ctx.jsonRawCheckpoint) == 0 {
		return
	}

	ctx.jsonRaw = make([]byte, len(ctx.jsonRawCheckpoint))
	copy(ctx.jsonRaw, ctx.jsonRawCheckpoint)
}

// AddBuiltInVars adds given pattern to the builtInVars
func (ctx *Context) AddBuiltInVars(pattern string) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	builtInVarsCopy := ctx.builtInVars
	ctx.builtInVars = append(builtInVarsCopy, pattern)
}

func (ctx *Context) getBuiltInVars() []string {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	vars := ctx.builtInVars
	return vars
}
