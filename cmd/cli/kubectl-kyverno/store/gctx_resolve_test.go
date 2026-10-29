package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestResolveGlobalContextMockData(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	raw := func(v interface{}) *runtime.RawExtension {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return &runtime.RawExtension{Raw: b}
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
		entry := v1alpha1.GlobalContextEntryValue{Name: "g"}
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
			Data: nil,
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

	t.Run("CEL expression projection", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Data: raw(map[string]interface{}{
				"deployments": []interface{}{
					map[string]interface{}{"name": "a"},
					map[string]interface{}{"name": "b"},
				},
			}),
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "count", Expression: "object.deployments.size()"},
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
		count, ok := m["count"].(int64)
		if !ok || count != 2 {
			t.Fatalf("count: %#v", m["count"])
		}
	})

	t.Run("CEL expression and JMESPath projections together", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Data: raw(map[string]interface{}{
				"deployments": []interface{}{
					map[string]interface{}{"name": "a"},
				},
			}),
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "fromCEL", Expression: "object.deployments.size()"},
				{Name: "fromJMESPath", Path: "deployments"},
			},
		}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		m := got.(map[string]interface{})
		if m["fromCEL"].(int64) != 1 {
			t.Fatalf("fromCEL: %#v", m["fromCEL"])
		}
		if items, ok := m["fromJMESPath"].([]interface{}); !ok || len(items) != 1 {
			t.Fatalf("fromJMESPath: %#v", m["fromJMESPath"])
		}
	})

	t.Run("CEL projections share one budget across the whole entry", func(t *testing.T) {
		// Each evaluation costs ~600,000 of the 3,000,000 background ceiling (same blob-size
		// math as pkg/cel/compiler's TestBudgetRefusalArrivesWithAUsableValue). If the budget
		// were established fresh per projection instead of once per entry, six projections
		// would each get their own 3,000,000 ceiling and never refuse.
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Data: raw(map[string]interface{}{"blob": strings.Repeat("b", 1_000_000)}),
		}
		for i := 0; i < 6; i++ {
			entry.Projections = append(entry.Projections, v1alpha1.GlobalContextProjection{
				Name:       fmt.Sprintf("p%d", i),
				Expression: "(object.blob + object.blob).size()",
			})
		}
		_, err := resolveGlobalContextMockData(jp, entry)
		if err == nil {
			t.Fatal("expected the shared per-entry budget to be exceeded")
		}
		if !errors.Is(err, requestbudget.ErrBudgetExceeded) {
			t.Fatalf("expected a budget refusal, got %v", err)
		}
	})

	t.Run("invalid CEL expression", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Data: raw(map[string]interface{}{"a": 1}),
			Projections: []v1alpha1.GlobalContextProjection{
				{Name: "x", Expression: "object.sizee()"},
			},
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
		// Empty but non-nil Resources slice → routes to resolveResourcesMockData → returns empty []interface{}
		got, err := resolveGlobalContextMockData(jp, entry)
		if err != nil {
			t.Fatal(err)
		}
		list, ok := got.([]interface{})
		if !ok {
			t.Fatalf("expected []interface{}, got %T", got)
		}
		if len(list) != 0 {
			t.Fatalf("expected empty list, got %v", list)
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

	t.Run("null resource is rejected", func(t *testing.T) {
		entry := v1alpha1.GlobalContextEntryValue{
			Name: "g",
			Resources: []runtime.RawExtension{
				{Raw: []byte(`null`)},
			},
		}
		_, err := resolveGlobalContextMockData(jp, entry)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
