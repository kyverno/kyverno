package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
		v, err := evaluateMockProjection(jp, entry.Name, p, root)
		if err != nil {
			return nil, err
		}
		out[p.Name] = v
	}
	return out, nil
}

// evaluateMockProjection evaluates one mock projection against root, mirroring the real
// GlobalContextEntryProjection's exactly-one-of jmesPath/expression contract: Path via
// jp.Search, Expression via the same compiled-CEL-program path
// (compiler.CompileGlobalContextProjection + gctxstore.Projection.Evaluate) the real
// GlobalContextEntry controller and `kyverno apply`'s buildGlobalContextStore use, so a
// mocked CEL projection behaves identically to a live one.
func evaluateMockProjection(jp jmespath.Interface, entryName string, p v1alpha1.GlobalContextProjection, root interface{}) (interface{}, error) {
	if p.Expression != "" {
		prog, errs := compiler.CompileGlobalContextProjection(field.NewPath("expression"), p.Expression)
		if len(errs) > 0 {
			return nil, fmt.Errorf("globalContextEntries %q projection %q expression %q: %w", entryName, p.Name, p.Expression, errs.ToAggregate())
		}
		ctx, release := requestbudget.EnsureBackground(context.Background())
		defer release()
		v, err := (gctxstore.Projection{Name: p.Name, Program: prog}).Evaluate(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("globalContextEntries %q projection %q expression %q: %w", entryName, p.Name, p.Expression, err)
		}
		return v, nil
	}
	v, err := jp.Search(p.Path, root)
	if err != nil {
		return nil, fmt.Errorf("globalContextEntries %q projection %q path %q: %w", entryName, p.Name, p.Path, err)
	}
	return v, nil
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
		if obj == nil {
			return nil, fmt.Errorf("resources[%d]: resource must be a JSON object, got null", i)
		}
		list = append(list, obj)
	}
	return list, nil
}
