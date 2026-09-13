package context

import (
	cont "context"
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"

	jsoniter "github.com/json-iterator/go"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/jsonutils"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
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
	AddUserInfo(userInfo kyvernov2.RequestInfo) error

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

	// Checkpoint records a pending checkpoint of the current internal state
	// and pushes it into a stack of stored states. The deep copy is taken
	// lazily, on the first subsequent write, rather than at call time.
	Checkpoint()

	// Restore sets the internal state to the last checkpoint, and removes the checkpoint.
	Restore()

	// Reset sets the internal state to the last checkpoint, but does not remove the checkpoint.
	Reset()

	// AddJSON  merges the json map with context
	addJSON(dataMap map[string]interface{}, overwriteMaps bool) error
}

// DefaultMaxContextSize is the default maximum size of context data in bytes (2MB)
const DefaultMaxContextSize = 2 * 1024 * 1024

// ContextSizeLimitExceededError is returned when context size exceeds the limit
type ContextSizeLimitExceededError struct {
	Size  int64
	Limit int64
}

func (e ContextSizeLimitExceededError) Error() string {
	return fmt.Sprintf("context size limit exceeded: %d bytes exceeds limit of %d bytes", e.Size, e.Limit)
}

// Context stores the data resources as JSON
type context struct {
	jp jmespath.Interface
	// jsonRaw is the live state graph. It must never be mutated in place
	// except through addJSON/clearLeafValue, and always after
	// materializeCheckpoints has been called first (see
	// materializeCheckpoints). Query()/QueryOperation() results alias this
	// graph and must not be mutated by callers.
	//
	// This holds today without any change outside this package:
	// pkg/engine/variables' SubstituteAll* family is the only consumer of
	// resolved variables that can hand a Query result to caller-mutable
	// code, and it always drives pkg/engine/jsonutils' TraverseJSON, whose
	// own recursion deep-copies anything a leaf action returns (map or
	// slice) before merging it back into the substituted document — see
	// pkg/engine/jsonutils/traverse_test.go's
	// Test_TraverseJSONCopiesLeafActionContainerResults, which pins that
	// property directly, and pkg/engine/variables' substitution_boundary_test.go,
	// which pins it end to end through a real validate-rule call sequence
	// (SubstituteAll -> ExpandInMetadata -> validate.MatchPattern). The
	// residual invariant is narrower than "no caller mutates a Query
	// result": it is "no caller feeds a raw Query()/QueryOperation() result
	// into an in-place mutator without going through SubstituteAll first."
	// The direct Query/QueryOperation callers outside this package are
	// audited read-only: pkg/engine/utils/foreach.go (EvaluateList,
	// elements re-enter only via AddElement),
	// pkg/engine/handlers/validation/validate_manifest.go (immediately
	// json.Marshal'd), the "target" existence probe in
	// pkg/engine/variables/vars.go (result discarded), and the CLI's
	// cmd/cli/kubectl-kyverno/processor/policy_processor.go:858-903
	// (each Query result is wrapped into a new unstructured.Unstructured
	// and set via WithNewResource/WithOldResource, not mutated in place).
	// A new direct-Query caller that mutates its result in place,
	// bypassing SubstituteAll, would violate this invariant — that is a
	// stop-and-escalate finding for review, not something to fix
	// silently here.
	jsonRaw map[string]interface{}
	// jsonRawCheckpoints is a stack of checkpoints. A nil entry is a
	// "pending" (unmaterialized) checkpoint that represents the live state
	// at the time Checkpoint() was called; a non-nil entry is a
	// materialized, fully independent deep copy with today's exact
	// semantics. See materializeCheckpoints for the invariants that make
	// this safe.
	jsonRawCheckpoints []map[string]interface{}
	images             map[string]map[string]apiutils.ImageInfo
	operation          kyvernov1.AdmissionOperation
	deferred           DeferredLoaders
	contextSize        int64
	maxContextSize     int64
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
		maxContextSize:     DefaultMaxContextSize,
	}
}

// NewContextWithMaxSize returns a new context with a specified maximum context size
func NewContextWithMaxSize(jp jmespath.Interface, maxSize int64) Interface {
	return &context{
		jp:                 jp,
		jsonRaw:            map[string]interface{}{},
		jsonRawCheckpoints: make([]map[string]interface{}, 0),
		deferred:           NewDeferredLoaders(),
		maxContextSize:     maxSize,
	}
}

// addJSON merges json data
func (ctx *context) addJSON(dataMap map[string]interface{}, overwriteMaps bool) error {
	ctx.materializeCheckpoints()
	mergeMaps(dataMap, ctx.jsonRaw, overwriteMaps)
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

	if err := addToContext(ctx, mapObj, false, "request"); err != nil {
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
		return addToContext(ctx, value, false, fields...)
	}
}

func (ctx *context) AddContextEntry(name string, dataRaw []byte) error {
	if err := ctx.checkContextSizeLimit(int64(len(dataRaw))); err != nil {
		return err
	}
	var data interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	ctx.contextSize += int64(len(dataRaw))
	return addToContext(ctx, data, false, name)
}

func (ctx *context) ReplaceContextEntry(name string, dataRaw []byte) error {
	if err := ctx.checkContextSizeLimit(int64(len(dataRaw))); err != nil {
		return err
	}
	var data interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	// Adding a nil entry to clean out any existing data in the context with the entry name
	if err := addToContext(ctx, nil, false, name); err != nil {
		logger.Error(err, "unable to replace context entry", "context entry name", name)
		return err
	}
	ctx.contextSize += int64(len(dataRaw))
	return addToContext(ctx, data, false, name)
}

// AddResource data at path: request.object
func (ctx *context) AddResource(data map[string]interface{}) error {
	ctx.materializeCheckpoints()
	clearLeafValue(ctx.jsonRaw, "request", "object")
	return addToContext(ctx, data, false, "request", "object")
}

// AddOldResource data at path: request.oldObject
func (ctx *context) AddOldResource(data map[string]interface{}) error {
	ctx.materializeCheckpoints()
	clearLeafValue(ctx.jsonRaw, "request", "oldObject")
	return addToContext(ctx, data, false, "request", "oldObject")
}

// AddTargetResource adds data at path: target
func (ctx *context) SetTargetResource(data map[string]interface{}) error {
	ctx.materializeCheckpoints()
	clearLeafValue(ctx.jsonRaw, "target")
	return addToContext(ctx, data, false, "target")
}

// AddOperation data at path: request.operation
func (ctx *context) AddOperation(data string) error {
	if err := addToContext(ctx, data, false, "request", "operation"); err != nil {
		return err
	}

	ctx.operation = kyvernov1.AdmissionOperation(data)
	return nil
}

// AddUserInfo adds userInfo at path request.userInfo
func (ctx *context) AddUserInfo(userRequestInfo kyvernov2.RequestInfo) error {
	if data, err := toUnstructured(&userRequestInfo); err == nil {
		return addToContext(ctx, data, false, "request")
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
	if err := ctx.addJSON(data, false); err != nil {
		return err
	}

	logger.V(4).Info("Adding service account", "service account name", saName, "service account namespace", saNamespace)
	return nil
}

// AddNamespace merges resource json under request.namespace
func (ctx *context) AddNamespace(namespace string) error {
	return addToContext(ctx, namespace, false, "request", "namespace")
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
	return addToContext(ctx, data, true)
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
	return addToContext(ctx, data, false, "image")
}

func (ctx *context) AddImageInfos(resource *unstructured.Unstructured, cfg config.Configuration) error {
	imageInfoLoader := &ImageInfoLoader{
		resource: resource,
		eCtx:     ctx,
		cfg:      cfg,
	}
	dl, err := NewDeferredLoader("images", imageInfoLoader, logger)
	if err != nil {
		return err
	}
	if toggle.FromContext(cont.Background()).EnableDeferredLoading() {
		if err := ctx.AddDeferredLoader(dl); err != nil {
			return err
		}
	} else {
		if err := imageInfoLoader.LoadData(); err != nil {
			return err
		}
	}
	return nil
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
	return addToContext(ctx, utm, false, "images")
}

type ImageInfoLoader struct {
	resource  *unstructured.Unstructured
	hasLoaded bool
	eCtx      *context
	cfg       config.Configuration
}

func (l *ImageInfoLoader) HasLoaded() bool {
	return l.hasLoaded
}

func (l *ImageInfoLoader) LoadData() error {
	images, err := apiutils.ExtractImagesFromResource(*l.resource, nil, l.cfg)
	if err != nil {
		return err
	}

	return l.eCtx.addImageInfos(images)
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
	// force load of image info from deferred loader
	if len(ctx.images) == 0 {
		if err := ctx.loadDeferred("images"); err != nil {
			return map[string]map[string]apiutils.ImageInfo{}
		}
	}
	return ctx.images
}

// Checkpoint records a pending checkpoint of the current internal state.
// No copy is made here: the checkpoint is lazily materialized as a full
// independent deep copy on the first subsequent write (see
// materializeCheckpoints). If no write ever follows, Restore/Reset are
// content no-ops and Checkpoint is effectively free.
func (ctx *context) Checkpoint() {
	ctx.jsonRawCheckpoints = append(ctx.jsonRawCheckpoints, nil)
}

// materializeCheckpoints converts every pending (nil) checkpoint into a
// full independent deep copy of the current state. It MUST be called
// before any mutation of the jsonRaw graph (see the jsonRaw invariant
// comment on the context struct). Each pending entry gets its OWN
// copyContext copy — entries must never share subtrees with each other
// or with jsonRaw, since a later write could otherwise reach and corrupt
// a checkpoint through a shared node.
//
// Invariant I1 (pending suffix): at all times the stack has the shape
// [materialized..., pending...] — all nil entries form a contiguous
// suffix at the top. Checkpoint appends nil at the top (suffix
// preserved); this function converts the entire nil suffix to non-nil
// (suffix becomes empty); Restore pops the top (a suffix minus its top
// element is still a suffix); Reset changes no entry's status. The state
// "outer pending, inner materialized" is therefore unreachable and must
// not be special-cased here.
//
// Invariant I2 (pending = live): every pending checkpoint represents a
// state byte-identical to the current jsonRaw, because no write can
// execute between a Checkpoint() call and this function without first
// materializing all pendings. So copyContext(jsonRaw) here produces
// exactly the bytes an eager Checkpoint() would have captured for each
// pending entry.
func (ctx *context) materializeCheckpoints() {
	for i := len(ctx.jsonRawCheckpoints) - 1; i >= 0; i-- {
		if ctx.jsonRawCheckpoints[i] != nil {
			break // invariant I1: pendings are a contiguous top suffix
		}
		ctx.jsonRawCheckpoints[i] = ctx.copyContext(ctx.jsonRaw)
	}
}

// copyContext returns a fully independent deep copy of in: every value is
// copied via runtime.DeepCopyJSONValue, with no distinction based on
// ReservedKeys. This full independence is what makes materialized
// checkpoints immune to alias injection (see #14526 and the
// Test_CheckpointNotCorruptedBy* suite) — a checkpoint shares zero nodes
// with the live graph, so a later write that installs a live Query()
// result by reference anywhere in jsonRaw can never reach a checkpoint.
func (ctx *context) copyContext(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = runtime.DeepCopyJSONValue(v)
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

	if jsonRawCheckpoint == nil {
		// Pending (unmaterialized) checkpoint: by invariant I2 its state
		// already equals the current jsonRaw (no write has happened since
		// Checkpoint() was called), so both Restore and Reset are content
		// no-ops. Restore still pops the stack entry to keep depth in
		// sync with eager; Reset leaves the entry pending (it must not
		// pop, and must not materialize — the next write will do that).
		if restore {
			ctx.jsonRawCheckpoints = ctx.jsonRawCheckpoints[:n]
		}
		return true
	}

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

// checkContextSizeLimit checks if adding additionalSize bytes would exceed the context size limit
func (ctx *context) checkContextSizeLimit(additionalSize int64) error {
	if ctx.maxContextSize > 0 && ctx.contextSize+additionalSize > ctx.maxContextSize {
		return ContextSizeLimitExceededError{
			Size:  ctx.contextSize + additionalSize,
			Limit: ctx.maxContextSize,
		}
	}
	return nil
}
