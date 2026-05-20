package store

import (
	"encoding/json"
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"k8s.io/apimachinery/pkg/runtime"
)

var defaultGctxJMESPath = jmespath.New(config.NewDefaultConfiguration(false))

func ResolveGlobalContextMockData(entry v1alpha1.GlobalContextEntryValue) (interface{}, error) {
	return resolveGlobalContextMockData(defaultGctxJMESPath, entry)
}

func resolveGlobalContextMockData(jp jmespath.Interface, entry v1alpha1.GlobalContextEntryValue) (interface{}, error) {
	// Route to the resources path when inline K8s manifests are provided.
	if entry.Resources != nil {
		return resolveResourcesMockData(jp, entry)
	}
	return resolveDataMockData(jp, entry)
}

// resolveDataMockData handles the existing data: path (arbitrary JSON root).
func resolveDataMockData(jp jmespath.Interface, entry v1alpha1.GlobalContextEntryValue) (interface{}, error) {
	if entry.Data == nil {
		if len(entry.Projections) > 0 {
			return nil, fmt.Errorf("globalContextEntries %q: data is required when projections are set", entry.Name)
		}
		return nil, nil
	}
	root, err := v1alpha1.RawExtensionToObject(*entry.Data)
	if err != nil {
		return nil, fmt.Errorf("globalContextEntries %q: %w", entry.Name, err)
	}
	if root == nil {
		return nil, nil
	}
	return applyFieldPathAndProjections(jp, entry, root)
}

// resolveResourcesMockData decodes inline resources to []interface{},
// matching the shape returned by the real k8sresource entry.
func resolveResourcesMockData(jp jmespath.Interface, entry v1alpha1.GlobalContextEntryValue) (interface{}, error) {
	list, err := rawExtensionListToObjects(entry.Resources)
	if err != nil {
		return nil, fmt.Errorf("globalContextEntries %q resources: %w", entry.Name, err)
	}
	return applyFieldPathAndProjections(jp, entry, list)
}

// applyFieldPathAndProjections applies the optional fieldPath and projections
// to the resolved root data. Shared by both data and resources paths.
func applyFieldPathAndProjections(jp jmespath.Interface, entry v1alpha1.GlobalContextEntryValue, root interface{}) (interface{}, error) {
	if entry.FieldPath != "" {
		var err error
		root, err = jp.Search(entry.FieldPath, root)
		if err != nil {
			return nil, fmt.Errorf("globalContextEntries %q fieldPath: %w", entry.Name, err)
		}
	}
	if len(entry.Projections) == 0 {
		return root, nil
	}
	out := make(map[string]interface{}, len(entry.Projections))
	for _, p := range entry.Projections {
		if p.Name == "" {
			return nil, fmt.Errorf("globalContextEntries %q projection name must not be empty", entry.Name)
		}
		v, err := jp.Search(p.Path, root)
		if err != nil {
			return nil, fmt.Errorf("globalContextEntries %q projection %q path %q: %w", entry.Name, p.Name, p.Path, err)
		}
		out[p.Name] = v
	}
	return out, nil
}

// rawExtensionListToObjects decodes []RawExtension into []interface{}.
func rawExtensionListToObjects(resources []runtime.RawExtension) ([]interface{}, error) {
	list := make([]interface{}, 0, len(resources))
	for i, r := range resources {
		if len(r.Raw) == 0 {
			return nil, fmt.Errorf("resources[%d]: empty resource", i)
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(r.Raw, &obj); err != nil {
			return nil, fmt.Errorf("resources[%d]: invalid JSON: %w", i, err)
		}
		list = append(list, obj)
	}
	return list, nil
}
