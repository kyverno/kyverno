package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EvalInterface is used to query and inspect context data
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

	// Checkpoint creates a copy of the current internal state and pushes it into a stack of stored states.
	Checkpoint()

	// Restore sets the internal state to the last checkpoint, and removes the checkpoint.
	Restore()

	// Reset sets the internal state to the last checkpoint, but does not remove the checkpoint.
	Reset()
}

// Loader fetches or produces data and loads it into the context. A loader is created for each
// context entry (e.g. `context.variable`, `context.apiCall`, etc.)
// Loaders are invoked lazily based on variable lookups. Loaders may be invoked multiple times to
// handle checkpoints and restores that occur when processing loops. A loader that fetches remote
// data should be able to handle multiple invocations in an optimal manner by mantaining internal
// state and caching remote data. For example, if an API call is made the data retrieved can be
// stored so that it can be saved in the outer context when a restore is performed.
type Loader interface {
	// Load data fetches or produces data and stores it in the context
	LoadData() error
	// Has loaded indicates if the loader has previously
	// executed and stored data in a context
	HasLoaded() bool
}

// DeferredLoader wraps a Loader and implements context specific behaviors.
// A `level` is used to track the checkpoint level at which the loader was
// created. If the level when loading occurs matches the loader's creation
// level, the loader is discarded after execution. Otherwise, the loader is
// retained so that it can be applied to the prior level when the checkpoint
// is restored or reset.
type DeferredLoader interface {
	Name() string
	Matches(query string) bool
	HasLoaded() bool
	LoadData() error
}

// LeveledLoader is a DeferredLoader with a Level
type LeveledLoader interface {
	// Level provides the declaration level for the DeferredLoader
	Level() int
	DeferredLoader
}

// DeferredLoaders manages a list of DeferredLoader instances
type DeferredLoaders interface {
	Add(loader DeferredLoader, level int)
	LoadMatching(query string, level int) error
	Reset(removeCheckpoint bool, level int)
}
