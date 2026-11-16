package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestResolveGCEResourceFiles(t *testing.T) {
	// Create a temp dir with a YAML file containing two documents.
	dir := filepath.Join(t.TempDir(), "test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep-1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-1
`
	if err := os.WriteFile(filepath.Join(dir, "resources.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("loads multi-doc YAML and populates Resources", func(t *testing.T) {
		entries := []v1alpha1.GlobalContextEntryValue{{
			Name:          "g",
			ResourceFiles: []string{"resources.yaml"},
		}}
		resolved, err := ResolveGCEResourceFiles(nil, dir, entries)
		if err != nil {
			t.Fatal(err)
		}
		if len(resolved) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(resolved))
		}
		if len(resolved[0].Resources) != 2 {
			t.Fatalf("expected 2 resources, got %d", len(resolved[0].Resources))
		}
		if len(resolved[0].ResourceFiles) != 0 {
			t.Fatal("expected ResourceFiles to be cleared")
		}

		// Verify the first resource is a Deployment
		obj, err := v1alpha1.RawExtensionToObject(resolved[0].Resources[0])
		if err != nil {
			t.Fatal(err)
		}
		m := obj.(map[string]interface{})
		if m["kind"] != "Deployment" {
			t.Fatalf("expected Deployment, got %v", m["kind"])
		}

		// Verify the second resource is a ConfigMap
		obj2, err := v1alpha1.RawExtensionToObject(resolved[0].Resources[1])
		if err != nil {
			t.Fatal(err)
		}
		m2 := obj2.(map[string]interface{})
		if m2["kind"] != "ConfigMap" {
			t.Fatalf("expected ConfigMap, got %v", m2["kind"])
		}
	})

	t.Run("skips entries without resourceFiles", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]interface{}{"x": 1})
		entries := []v1alpha1.GlobalContextEntryValue{{
			Name: "g",
			Data: &runtime.RawExtension{Raw: raw},
		}}
		resolved, err := ResolveGCEResourceFiles(nil, dir, entries)
		if err != nil {
			t.Fatal(err)
		}
		if resolved[0].Data == nil || len(resolved[0].Data.Raw) == 0 {
			t.Fatal("expected Data to be preserved")
		}
	})

	t.Run("non-nil empty resourceFiles resolves to empty Resources list", func(t *testing.T) {
		entries := []v1alpha1.GlobalContextEntryValue{{
			Name:          "g",
			ResourceFiles: []string{}, // explicitly set but empty — must NOT be skipped
		}}
		resolved, err := ResolveGCEResourceFiles(nil, dir, entries)
		if err != nil {
			t.Fatal(err)
		}
		if resolved[0].ResourceFiles != nil {
			t.Fatal("expected ResourceFiles to be cleared")
		}
		if resolved[0].Resources == nil {
			t.Fatal("expected Resources to be an empty (non-nil) slice, not nil")
		}
		if len(resolved[0].Resources) != 0 {
			t.Fatalf("expected 0 resources, got %d", len(resolved[0].Resources))
		}
	})

	t.Run("error on missing file", func(t *testing.T) {
		entries := []v1alpha1.GlobalContextEntryValue{{
			Name:          "g",
			ResourceFiles: []string{"nonexistent.yaml"},
		}}
		_, err := ResolveGCEResourceFiles(nil, dir, entries)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
