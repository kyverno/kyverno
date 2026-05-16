package store

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestResolveGlobalContextMockData(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	raw := func(v interface{}) runtime.RawExtension {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return runtime.RawExtension{Raw: b}
	}

	t.Run("projections", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Data: raw(map[string]interface{}{
				"deployments": []interface{}{
					map[string]interface{}{"name": "a"},
				},
			}),
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "items", Path: "deployments"},
			},
		}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		m, ok := got.(map[string]interface{})
		if !ok {
			t.Fatalf("got %T", got)
		}
		items, ok := m["items"].([]interface{})
		if !ok || len(items) != 1 {
			t.Fatalf("items: %#v", m["items"])
		}
	})

	t.Run("fieldPath then projections", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name:      "g",
			FieldPath: "root",
			Data: raw(map[string]interface{}{
				"root": map[string]interface{}{
					"deployments": []interface{}{"x", "y"},
				},
			}),
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "items", Path: "deployments"},
			},
		}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		m := got.(map[string]interface{})
		if len(m["items"].([]interface{})) != 2 {
			t.Fatalf("%#v", got)
		}
	})

	t.Run("empty data no projections", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{Name: "g", Data: runtime.RawExtension{}}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("want nil, got %#v", got)
		}
	})

	t.Run("projections require data", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Data: runtime.RawExtension{},
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "x", Path: "y"},
			},
		}
		_, err := resolveGlobalContextMockData(jp, entry)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid fieldPath", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name:      "g",
			FieldPath: "this is not valid jmespath {{{",
			Data:      raw(map[string]interface{}{"a": 1}),
		}
		_, err := resolveGlobalContextMockData(jp, entry)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestResolveResourcesMockData(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	rawResource := func(v map[string]interface{}) runtime.RawExtension {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return runtime.RawExtension{Raw: b}
	}

	dep1 := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "dep-1", "namespace": "default"},
		"spec":       map[string]interface{}{"replicas": float64(1)},
	}
	dep2 := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "dep-2", "namespace": "default"},
		"spec":       map[string]interface{}{"replicas": float64(3)},
	}

	t.Run("resources returns []interface{} matching real k8sresource shape", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Resources: []runtime.RawExtension{
				rawResource(dep1),
				rawResource(dep2),
			},
		}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		list, ok := got.([]interface{})
		if !ok {
			t.Fatalf("expected []interface{}, got %T", got)
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 resources, got %d", len(list))
		}
		// Each element should be map[string]interface{} matching unstructured.Object
		for i, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				t.Fatalf("resources[%d]: expected map[string]interface{}, got %T", i, item)
			}
			if m["kind"] != "Deployment" {
				t.Fatalf("resources[%d]: expected kind=Deployment, got %v", i, m["kind"])
			}
		}
	})

	t.Run("resources with projection extracts metadata names", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Resources: []runtime.RawExtension{
				rawResource(dep1),
				rawResource(dep2),
			},
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "names", Path: "[*].metadata.name"},
			},
		}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		m, ok := got.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map[string]interface{}, got %T", got)
		}
		names, ok := m["names"].([]interface{})
		if !ok {
			t.Fatalf("expected []interface{} for names, got %T", m["names"])
		}
		if len(names) != 2 || names[0] != "dep-1" || names[1] != "dep-2" {
			t.Fatalf("unexpected names: %v", names)
		}
	})

	t.Run("resources with fieldPath", func(t *testing.T) {
		// fieldPath on a list: "[0]" picks the first element
		entry := v1alpha1.GlobalContextEntryValue{
			Name:      "g",
			FieldPath: "[0]",
			Resources: []runtime.RawExtension{
				rawResource(dep1),
				rawResource(dep2),
			},
		}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		m, ok := got.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map[string]interface{}, got %T", got)
		}
		meta := m["metadata"].(map[string]interface{})
		if meta["name"] != "dep-1" {
			t.Fatalf("expected dep-1, got %v", meta["name"])
		}
	})

	t.Run("resources with length projection", func(t *testing.T) {
		// Mimics: globalReference jmesPath "items | length(@)"
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Resources: []runtime.RawExtension{
				rawResource(dep1),
				rawResource(dep2),
			},
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "count", Path: "length(@)"},
			},
		}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		m := got.(map[string]interface{})
		count, ok := m["count"].(float64)
		if !ok {
			t.Fatalf("expected float64, got %T: %v", m["count"], m["count"])
		}
		if int(count) != 2 {
			t.Fatalf("expected 2, got %v", count)
		}
	})

	t.Run("empty resources list", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name:      "g",
			Resources: []runtime.RawExtension{},
		}
		// Empty resources → routes to data path (no resources, no data → nil)
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("invalid resource JSON", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Resources: []runtime.RawExtension{
				{Raw: []byte(`{invalid`)},
			},
		}
		_, err := resolveGlobalContextMockData(jp, entry)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
