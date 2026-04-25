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
}
