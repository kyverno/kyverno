package context

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//Interface to manage context operations
type Interface interface {

	// AddRequest marshals and adds the admission request to the context
	AddRequest(request *v1beta1.AdmissionRequest) error

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

//EvalInterface is used to query and inspect context data
type EvalInterface interface {

	// Query accepts a JMESPath expression and returns matching data
	Query(query string) (interface{}, error)

	// HasChanged accepts a JMESPath expression and compares matching data in the
	// request.object and request.oldObject context fields. If the data has changed
	// it return `true`. If the data has not changed it returns false. If either
	// request.object or request.oldObject are not found, an error is returned.
	HasChanged(jmespath string) (bool, error)
}

//Context stores the data resources as JSON
type Context struct {
	mutex              sync.RWMutex
	jsonRaw            []byte
	jsonRawCheckpoints [][]byte
	builtInVars        []string
	images             *Images
	log                logr.Logger
}

//NewContext returns a new context
// builtInVars is the list of known variables (e.g. serviceAccountName)
func NewContext(builtInVars ...string) *Context {
	ctx := Context{
		jsonRaw:            []byte(`{}`), // empty json struct
		builtInVars:        builtInVars,
		log:                log.Log.WithName("context"),
		jsonRawCheckpoints: make([][]byte, 0),
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
	ctx.jsonRaw, err = jsonpatch.MergeMergePatches(ctx.jsonRaw, dataRaw)

	if err != nil {
		ctx.log.Error(err, "failed to merge JSON data")
		return err
	}
	return nil
}

// AddJSONObject merges json data
func (ctx *Context) AddJSONObject(jsonData interface{}) error {
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return err
	}

	return ctx.AddJSON(jsonBytes)
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

//AddResourceInOldObject data at path: request.oldObject
func (ctx *Context) AddResourceInOldObject(dataRaw []byte) error {

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
			OldObject interface{} `json:"oldObject"`
		}{
			OldObject: data,
		},
	}

	objRaw, err := json.Marshal(modifiedResource)
	if err != nil {
		ctx.log.Error(err, "failed to marshal the resource")
		return err
	}
	return ctx.AddJSON(objRaw)
}

func (ctx *Context) AddResourceAsObject(data interface{}) error {
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
	ctx.log.V(4).Info("Adding user info logs", "userRequestInfo", userRequestInfo)
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
	ctx.log.V(4).Info("Adding service account", "service account name", saName, "service account namespace", saNamespace)
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

func (ctx *Context) AddImageInfo(resource *unstructured.Unstructured) error {
	initContainersImgs, containersImgs := extractImageInfo(resource, ctx.log)
	if len(initContainersImgs) == 0 && len(containersImgs) == 0 {
		return nil
	}

	images := newImages(initContainersImgs, containersImgs)
	if images == nil {
		return nil
	}

	ctx.images = images
	imagesTag := struct {
		Images interface{} `json:"images"`
	}{
		Images: images,
	}

	objRaw, err := json.Marshal(imagesTag)
	if err != nil {
		return err
	}

	return ctx.AddJSON(objRaw)
}

func (ctx *Context) ImageInfo() *Images {
	return ctx.images
}

// Checkpoint creates a copy of the current internal state and
// pushes it into a stack of stored states.
func (ctx *Context) Checkpoint() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	jsonRawCheckpoint := make([]byte, len(ctx.jsonRaw))
	copy(jsonRawCheckpoint, ctx.jsonRaw)

	ctx.jsonRawCheckpoints = append(ctx.jsonRawCheckpoints, jsonRawCheckpoint)
}

// Restore sets the internal state to the last checkpoint, and removes the checkpoint.
func (ctx *Context) Restore() {
	ctx.reset(true)
}

// Reset sets the internal state to the last checkpoint, but does not remove the checkpoint.
func (ctx *Context) Reset() {
	ctx.reset(false)
}

func (ctx *Context) reset(remove bool) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if len(ctx.jsonRawCheckpoints) == 0 {
		return
	}

	n := len(ctx.jsonRawCheckpoints) - 1
	jsonRawCheckpoint := ctx.jsonRawCheckpoints[n]

	ctx.jsonRaw = make([]byte, len(jsonRawCheckpoint))
	copy(ctx.jsonRaw, jsonRawCheckpoint)

	if remove {
		ctx.jsonRawCheckpoints = ctx.jsonRawCheckpoints[:n]
	}
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
