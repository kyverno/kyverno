package store

import (
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

var defaultGctxJMESPath = jmespath.New(config.NewDefaultConfiguration(false))

// ResolveGlobalContextMockData builds the object passed to v1 globalReference and CEL globalContext
// from a test manifest entry: optional FieldPath narrows Data, then Projections (if any) become top-level keys.
func ResolveGlobalContextMockData(entry v1alpha1.GlobalContextEntryValue) (interface{}, error) {
	return resolveGlobalContextMockData(defaultGctxJMESPath, entry)
}

func resolveGlobalContextMockData(jp jmespath.Interface, entry v1alpha1.GlobalContextEntryValue) (interface{}, error) {
	root, err := v1alpha1.RawExtensionToObject(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("globalContextEntries %q: %w", entry.Name, err)
	}
	if len(entry.Projections) > 0 && root == nil {
		return nil, fmt.Errorf("globalContextEntries %q: data is required when projections are set", entry.Name)
	}
	if root == nil {
		return nil, nil
	}
	if entry.FieldPath != "" {
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
