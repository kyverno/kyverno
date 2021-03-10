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

// type imgInfo map[string]string

type imgInfo struct {
	RegistryURL string `json:"registryURL"`
	Name        string `json:"name"`
	Tag         string `json:"tag"`
}

type containerImage struct {
	Name      string
	Container imgInfo `json:"needtoreplace"`
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

//AddImageDetails checks if kind is pod or pod controller, then loads details about the image
func (ctx *Context) AddImageDetails(kindInterface interface{}, specInterface interface{}) error {
	containerImages := make(map[string]string)
	containerImgs := make(map[string]imgInfo)

	var initContainerImages []string
	var initContainerImgs []imgInfo

	kind := kindInterface.(string)
	spec := specInterface.(map[string]interface{})

	if kind == "Pod" {
		containersMap := spec["containers"].([]interface{})
		for _, v := range containersMap { //containers is a slice of maps where each map represents an image
			v2 := v.(map[string]interface{})
			imageString := v2["image"].(string)
			imageNameString := v2["name"].(string)
			containerImages[imageNameString] = imageString
		}
		initContainersMap := spec["initContainers"].([]interface{})
		for _, v := range initContainersMap { //containers is a slice of maps where each map represents an image
			v2 := v.(map[string]interface{})
			imageString := v2["image"].(string)
			//imageNameString := v2["name"].(string)
			initContainerImages = append(initContainerImages, imageString)
		}
	}

	if kind == "Deployment" || kind == "Job" || kind == "CronJob" || kind == "ReplicaSet" {
		template := spec["template"].(map[string]interface{})
		templateSpec := template["spec"].(map[string]interface{})
		containersMap := templateSpec["containers"].([]interface{})
		for _, v := range containersMap { //containers is a slice of maps where each map represents an image
			v2 := v.(map[string]interface{})
			imageString := v2["image"].(string)
			imageNameString := v2["name"].(string)
			containerImages[imageNameString] = imageString
		}
		initContainersMap := templateSpec["initContainers"].([]interface{})
		for _, v := range initContainersMap { //containers is a slice of maps where each map represents an image
			v2 := v.(map[string]interface{})
			imageString := v2["image"].(string)
			//imageNameString := v2["name"].(string)
			initContainerImages = append(initContainerImages, imageString)
		}
	}

	fmt.Println(containerImages)

	for imageName, image := range containerImages {
		var img imgInfo
		if strings.Contains(image, "/") {
			res := strings.Split(image, "/")
			img.RegistryURL = res[0]
			image = res[1]
		} else {
			img.RegistryURL = ""
		}
		if strings.Contains(image, ":") {
			res := strings.Split(image, ":")
			img.Name = res[0]
			img.Tag = res[1]
		} else {
			img.Name = image
			img.Tag = "latest"
		}
		containerImgs[imageName] = img
	}
	fmt.Println(containerImgs)

	fmt.Println(initContainerImages)

	for _, image := range initContainerImages {
		var img imgInfo
		if strings.Contains(image, "/") {
			res := strings.Split(image, "/")
			img.RegistryURL = res[0]
			image = res[1]
		} else {
			img.RegistryURL = ""
		}
		if strings.Contains(image, ":") {
			res := strings.Split(image, ":")
			img.Name = res[0]
			img.Tag = res[1]
		} else {
			img.Name = image
			img.Tag = "latest"
		}
		initContainerImgs = append(initContainerImgs, img)
	}
	fmt.Println(initContainerImgs)

	for k, v := range containerImgs {
		err := ctx.AddImageInfo(k, v)
		if err != nil {
			fmt.Println("===unexpected error ", err)
		}
	}

	// imgsObj := struct {
	// 	Images interface{} `json:"images"`
	// }{
	// 	Images: struct {
	// 		Containers     []imgInfo `json:"containers"`
	// 		InitContainers []imgInfo `json:"initContainers"`
	// 	}{
	// 		Containers:     containerImgs,
	// 		InitContainers: initContainerImgs,
	// 	},
	// }

	// fmt.Println("$$$$$")
	// fmt.Println(imgsObj)
	// fmt.Println("$$$$$")
	// imgsRaw, err := json.Marshal(imgsObj)
	// if err != nil {
	// 	ctx.log.Error(err, "failed to marshal the IMG")
	// 	return err
	// }
	// fmt.Println("#####")
	// fmt.Println(imgsRaw)
	// fmt.Println("#####")
	// fmt.Println("&&&&&")
	// fmt.Println(string(imgsRaw))
	// fmt.Println("&&&&&")
	// if err := ctx.AddJSON(imgsRaw); err != nil {
	// 	return err
	// }

	return nil
}

func newContainerImage(containerName string, imgInfo imgInfo) containerImage {
	return containerImage{
		Name:      containerName,
		Container: imgInfo,
	}
}
func (c containerImage) ReplaceJSONTag() map[string]interface{} {
	m, _ := json.Marshal(c.Container)

	var a interface{}
	json.Unmarshal(m, &a)
	b := a.(map[string]interface{})
	fmt.Println("===b", b)
	b[c.Name] = c.Container
	delete(b, "needtoreplace")

	return b
}

// AddImageInfo no lint
func (ctx *Context) AddImageInfo(k string, v imgInfo) error {

	containerImg := newContainerImage(k, v)

	replacedContainerImg := containerImg.ReplaceJSONTag()

	images := struct {
		Images interface{} `json:"images"`
	}{
		Images: struct {
			Container map[string]interface{} `json:"container"`
		}{
			Container: replacedContainerImg,
		},
	}

	objRaw, err := json.Marshal(images)
	if err != nil {
		fmt.Println("===failed to marshal images", err)
	}

	fmt.Println("===objRaw", string(objRaw))
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
