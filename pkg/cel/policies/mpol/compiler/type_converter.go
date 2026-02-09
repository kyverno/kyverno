package compiler

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/spec3"
)

// TypeConverterManager is an interface for a static version of the patch.TypeConverterManager without background processing
type TypeConverterManager interface {
	// GetTypeConverter returns a type converter for the given GVK
	GetTypeConverter(gvk schema.GroupVersionKind) managedfields.TypeConverter
}

func NewStaticTypeConverterManager(openapiClient openapi.Client) TypeConverterManager {
	return &staticTypeConverterManager{
		openapiClient:    openapiClient,
		typeConverterMap: make(map[schema.GroupVersion]typeConverterCacheEntry),
	}
}

type typeConverterCacheEntry struct {
	typeConverter managedfields.TypeConverter
	entry         openapi.GroupVersion
}

type staticTypeConverterManager struct {
	openapiClient    openapi.Client
	typeConverterMap map[schema.GroupVersion]typeConverterCacheEntry
	fetchedPaths     map[schema.GroupVersion]openapi.GroupVersion
	lock             sync.RWMutex
}

func (t *staticTypeConverterManager) fetchPaths() (map[schema.GroupVersion]openapi.GroupVersion, error) {
	if t.fetchedPaths != nil {
		return t.fetchedPaths, nil
	}

	paths, err := t.openapiClient.Paths()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch openapi paths: %w", err)
	}

	parsedPaths := make(map[schema.GroupVersion]openapi.GroupVersion, len(paths))
	for path, entry := range paths {
		if !strings.HasPrefix(path, "apis/") && !strings.HasPrefix(path, "api/") {
			continue
		}
		path = strings.TrimPrefix(path, "apis/")
		path = strings.TrimPrefix(path, "api/")

		gv, err := schema.ParseGroupVersion(path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse group version %q: %w", path, err)
		}

		parsedPaths[gv] = entry
	}

	t.fetchedPaths = parsedPaths

	return parsedPaths, nil
}

func (t *staticTypeConverterManager) GetTypeConverter(gvk schema.GroupVersionKind) managedfields.TypeConverter {
	gv := gvk.GroupVersion()

	fetchedPaths, err := t.fetchPaths()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to fetch paths from openapi client: %w", err))
		return nil
	}

	existing, entry, err := func() (managedfields.TypeConverter, openapi.GroupVersion, error) {
		t.lock.RLock()
		defer t.lock.RUnlock()

		// If schema is not supported by static type converter, ask discovery
		// for the schema
		entry, ok := fetchedPaths[gv]
		if !ok {
			// If we can't get the schema, we can't do anything
			return nil, nil, fmt.Errorf("no schema for %v", gvk)
		}

		// If the entry schema has not changed, used the same type converter
		if existing, ok := t.typeConverterMap[gv]; ok && existing.entry.ServerRelativeURL() == entry.ServerRelativeURL() {
			// If we have a type converter for this GVK, return it
			return existing.typeConverter, existing.entry, nil
		}

		return nil, entry, nil
	}()
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	} else if existing != nil {
		return existing
	}

	schBytes, err := entry.Schema(runtime.ContentTypeJSON)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to get schema for %v: %w", gvk, err))
		return nil
	}

	var sch spec3.OpenAPI
	if err := json.Unmarshal(schBytes, &sch); err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to unmarshal schema for %v: %w", gvk, err))
		return nil
	}

	// The schema has changed, or there is no entry for it, generate
	// a new type converter for this GV
	tc, err := managedfields.NewTypeConverter(sch.Components.Schemas, false)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to create type converter for %v: %w", gvk, err))
		return nil
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	t.typeConverterMap[gv] = typeConverterCacheEntry{
		typeConverter: tc,
		entry:         entry,
	}

	return tc
}
