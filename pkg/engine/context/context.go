package context

import (
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"

	jsoniter "github.com/json-iterator/go"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/jsonutils"
	"github.com/kyverno/kyverno/pkg/logging"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	logger       = logging.WithName("context")
	json         = jsoniter.ConfigCompatibleWithStandardLibrary
	ReservedKeys = regexp.MustCompile(`request|serviceAccountName|serviceAccountNamespace|element|elementIndex|@|images|image|([a-z_0-9]+\()[^{}]`)
)

// EvalInterface is used to query and inspect context data
// TODO: move to contextapi to prevent circular dependencies
type EvalInterface interface {
	// Query accepts a JMESPath expression and returns matching data
	Query(query string) (interface{}, error)

	// Operation returns the admission operation i.e. "request.operation"
	QueryOperation() string

	// HasChanged accepts a JMESPath expression and compares matching data in the
	// request.object and request.oldObject context fields. If the data has changed
	// it return `true`. If the data has not changed it returns false. If either
	// request.object or request.oldObject are not found, an error is returned.
	HasChanged(jmespath string) (bool, error)
}

// Interface to manage context operations
// TODO: move to contextapi to prevent circular dependencies
type Interface interface {
	EvalInterface

	// AddRequest marshals and adds the admission request to the context
	AddRequest(request admissionv1.AdmissionRequest) error

	// AddVariable adds a variable to the context
	AddVariable(key string, value interface{}) error

	// AddContextEntry adds a context entry to the context
	AddContextEntry(name string, dataRaw []byte) error

	// ReplaceContextEntry replaces a context entry to the context
	ReplaceContextEntry(name string, dataRaw []byte) error

	// AddResource merges resource json under request.object
	AddResource(data map[string]interface{}) error

	// AddOldResource merges resource json under request.oldObject
	AddOldResource(data map[string]interface{}) error

	// SetTargetResource merges resource json under target
	SetTargetResource(data map[string]interface{}) error

	// AddOperation merges operation under request.operation
	AddOperation(data string) error

	// AddUserInfo merges userInfo json under kyverno.userInfo
	AddUserInfo(userInfo kyvernov1beta1.RequestInfo) error

	// AddServiceAccount merges ServiceAccount types
	AddServiceAccount(userName string) error

	// AddNamespace merges resource json under request.namespace
	AddNamespace(namespace string) error

	// AddElement adds element info to the context
	AddElement(data interface{}, index, nesting int) error

	// AddImageInfo adds image info to the context
	AddImageInfo(info apiutils.ImageInfo, cfg config.Configuration) error

	// AddImageInfos adds image infos to the context
	AddImageInfos(resource *unstructured.Unstructured, cfg config.Configuration) error

	// AddDeferredLoader adds a loader that is executed on first use (query)
	// If deferred loading is disabled the loader is immediately executed.
	AddDeferredLoader(loader DeferredLoader) error

	// ImageInfo returns image infos present in the context
	ImageInfo() map[string]map[string]apiutils.ImageInfo

	// GenerateCustomImageInfo returns image infos as defined by a custom image extraction config
	// and updates the context
	GenerateCustomImageInfo(resource *unstructured.Unstructured, imageExtractorConfigs kyvernov1.ImageExtractorConfigs, cfg config.Configuration) (map[string]map[string]apiutils.ImageInfo, error)

	// Checkpoint creates a copy of the current internal state and pushes it into a stack of stored states.
	Checkpoint()

	// Restore sets the internal state to the last checkpoint, and removes the checkpoint.
	Restore()

	// Reset sets the internal state to the last checkpoint, but does not remove the checkpoint.
	Reset()

	// AddJSON  merges the json map with context
	addJSON(dataMap map[string]interface{}) error
}

// Context stores the data resources as JSON
type context struct {
	jp                 jmespath.Interface
	jsonRaw            map[string]interface{}
	jsonRawCheckpoints []map[string]interface{}
	images             map[string]map[string]apiutils.ImageInfo
	operation          kyvernov1.AdmissionOperation
	deferred           DeferredLoaders
}

// NewContext returns a new context
func NewContext(jp jmespath.Interface) Interface {
	return NewContextFromRaw(jp, map[string]interface{}{})
}

// NewContextFromRaw returns a new context initialized with raw data
func NewContextFromRaw(jp jmespath.Interface, raw map[string]interface{}) Interface {
	return &context{
		jp:                 jp,
		jsonRaw:            raw,
		jsonRawCheckpoints: make([]map[string]interface{}, 0),
		deferred:           NewDeferredLoaders(),
	}
}

// addJSON merges json data
func (ctx *context) addJSON(dataMap map[string]interface{}) error {
	mergeMaps(dataMap, ctx.jsonRaw)
	return nil
}

func (ctx *context) QueryOperation() string {
	if ctx.operation != "" {
		return string(ctx.operation)
	}

	if requestMap, val := ctx.jsonRaw["request"].(map[string]interface{}); val {
		if op, val := requestMap["operation"].(string); val {
			return op
		}
	}

	return ""
}

// AddRequest adds an admission request to context
func (ctx *context) AddRequest(request admissionv1.AdmissionRequest) error {
	// an AdmissionRequest needs to be marshaled / unmarshaled as
	// JSON to properly convert types of runtime.RawExtension
	mapObj, err := jsonutils.DocumentToUntyped(request)
	if err != nil {
		return err
	}

	if err := addToContext(ctx, mapObj, "request"); err != nil {
		return err
	}

	ctx.operation = kyvernov1.AdmissionOperation(request.Operation)
	return nil
}

func (ctx *context) AddVariable(key string, value interface{}) error {
	reader := csv.NewReader(strings.NewReader(key))
	reader.Comma = '.'
	if fields, err := reader.Read(); err != nil {
		return err
	} else {
		return addToContext(ctx, value, fields...)
	}
}

func (ctx *context) AddContextEntry(name string, dataRaw []byte) error {
	var data interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	return addToContext(ctx, data, name)
}

func (ctx *context) ReplaceContextEntry(name string, dataRaw []byte) error {
	var data interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	// Adding a nil entry to clean out any existing data in the context with the entry name
	if err := addToContext(ctx, nil, name); err != nil {
		logger.Error(err, "unable to replace context entry", "context entry name", name)
		return err
	}
	return addToContext(ctx, data, name)
}

// AddResource data at path: request.object
func (ctx *context) AddResource(data map[string]interface{}) error {
	clearLeafValue(ctx.jsonRaw, "request", "object")
	return addToContext(ctx, data, "request", "object")
}

// AddOldResource data at path: request.oldObject
func (ctx *context) AddOldResource(data map[string]interface{}) error {
	clearLeafValue(ctx.jsonRaw, "request", "oldObject")
	return addToContext(ctx, data, "request", "oldObject")
}

// AddTargetResource adds data at path: target
func (ctx *context) SetTargetResource(data map[string]interface{}) error {
	clearLeafValue(ctx.jsonRaw, "target")
	return addToContext(ctx, data, "target")
}

// AddOperation data at path: request.operation
func (ctx *context) AddOperation(data string) error {
	if err := addToContext(ctx, data, "request", "operation"); err != nil {
		return err
	}

	ctx.operation = kyvernov1.AdmissionOperation(data)
	return nil
}

// AddUserInfo adds userInfo at path request.userInfo
func (ctx *context) AddUserInfo(userRequestInfo kyvernov1beta1.RequestInfo) error {
	if data, err := toUnstructured(&userRequestInfo); err == nil {
		return addToContext(ctx, data, "request")
	} else {
		return err
	}
}

// AddServiceAccount removes prefix 'system:serviceaccount:' and namespace, then loads only SA name and SA namespace
func (ctx *context) AddServiceAccount(userName string) error {
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
	data := map[string]interface{}{
		"serviceAccountName":      saName,
		"serviceAccountNamespace": saNamespace,
	}
	if err := ctx.addJSON(data); err != nil {
		return err
	}

	logger.V(4).Info("Adding service account", "service account name", saName, "service account namespace", saNamespace)
	return nil
}

// AddNamespace merges resource json under request.namespace
func (ctx *context) AddNamespace(namespace string) error {
	return addToContext(ctx, namespace, "request", "namespace")
}

func (ctx *context) AddElement(data interface{}, index, nesting int) error {
	nestedElement := fmt.Sprintf("element%d", nesting)
	nestedElementIndex := fmt.Sprintf("elementIndex%d", nesting)
	data = map[string]interface{}{
		"element":          data,
		nestedElement:      data,
		"elementIndex":     int64(index),
		nestedElementIndex: int64(index),
	}
	return addToContext(ctx, data)
}

func (ctx *context) AddImageInfo(info apiutils.ImageInfo, cfg config.Configuration) error {
	data := map[string]interface{}{
		"reference":        info.Reference,
		"referenceWithTag": info.ReferenceWithTag,
		"registry":         info.Registry,
		"path":             info.Path,
		"name":             info.Name,
		"tag":              info.Tag,
		"digest":           info.Digest,
	}
	return addToContext(ctx, data, "image")
}

func (ctx *context) AddImageInfos(resource *unstructured.Unstructured, cfg config.Configuration) error {
	images, err := apiutils.ExtractImagesFromResource(*resource, nil, cfg)
	if err != nil {
		return err
	}

	return ctx.addImageInfos(images)
}

func (ctx *context) addImageInfos(images map[string]map[string]apiutils.ImageInfo) error {
	if len(images) == 0 {
		return nil
	}
	ctx.images = images
	utm, err := convertImagesToUnstructured(images)
	if err != nil {
		return err
	}

	logging.V(4).Info("updated image info", "images", utm)
	return addToContext(ctx, utm, "images")
}

func convertImagesToUnstructured(images map[string]map[string]apiutils.ImageInfo) (map[string]interface{}, error) {
	results := map[string]interface{}{}
	for containerType, v := range images {
		imgMap := map[string]interface{}{}
		for containerName := range v {
			imageInfo := v[containerName]
			img, err := toUnstructured(&imageInfo.ImageInfo)
			if err != nil {
				return nil, err
			}

			var pointer interface{} = imageInfo.Pointer
			img["jsonPointer"] = pointer

			imgMap[containerName] = img
		}

		results[containerType] = imgMap
	}

	return results, nil
}

func (ctx *context) GenerateCustomImageInfo(resource *unstructured.Unstructured, imageExtractorConfigs kyvernov1.ImageExtractorConfigs, cfg config.Configuration) (map[string]map[string]apiutils.ImageInfo, error) {
	images, err := apiutils.ExtractImagesFromResource(*resource, imageExtractorConfigs, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to extract images: %w", err)
	}

	if err := ctx.addImageInfos(images); err != nil {
		return nil, fmt.Errorf("failed to add images to context: %w", err)
	}

	return images, nil
}

func (ctx *context) ImageInfo() map[string]map[string]apiutils.ImageInfo {
	return ctx.images
}

// Checkpoint creates a copy of the current internal state and
// pushes it into a stack of stored states.
func (ctx *context) Checkpoint() {
	jsonRawCheckpoint := ctx.copyContext(ctx.jsonRaw)
	ctx.jsonRawCheckpoints = append(ctx.jsonRawCheckpoints, jsonRawCheckpoint)
}

func (ctx *context) copyContext(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		if ReservedKeys.MatchString(k) {
			out[k] = v
		} else {
			out[k] = runtime.DeepCopyJSONValue(v)
		}
	}

	return out
}

// Restore sets the internal state to the last checkpoint, and removes the checkpoint.
func (ctx *context) Restore() {
	ctx.reset(true)
}

// Reset sets the internal state to the last checkpoint, but does not remove the checkpoint.
func (ctx *context) Reset() {
	ctx.reset(false)
}

func (ctx *context) reset(restore bool) {
	if ctx.resetCheckpoint(restore) {
		ctx.deferred.Reset(restore, len(ctx.jsonRawCheckpoints))
	}
}

func (ctx *context) resetCheckpoint(restore bool) bool {
	if len(ctx.jsonRawCheckpoints) == 0 {
		return false
	}

	n := len(ctx.jsonRawCheckpoints) - 1
	jsonRawCheckpoint := ctx.jsonRawCheckpoints[n]

	if restore {
		ctx.jsonRawCheckpoints = ctx.jsonRawCheckpoints[:n]
		ctx.jsonRaw = jsonRawCheckpoint
	} else {
		ctx.jsonRaw = ctx.copyContext(jsonRawCheckpoint)
	}

	return true
}

func (ctx *context) AddDeferredLoader(dl DeferredLoader) error {
	ctx.deferred.Add(dl, len(ctx.jsonRawCheckpoints))
	return nil
}
