package attestations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sbom.json", `{"bomFormat":"CycloneDX"}`)
	writeFile(t, dir, "empty.json", ``)
	writeFile(t, dir, "invalid.json", `{not json}`)
	writeFile(t, dir, "emptyobj.json", `{}`)

	t.Run("nil entries", func(t *testing.T) {
		provider, err := Load(nil, dir, false, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider != nil {
			t.Fatalf("expected nil provider, got %v", provider)
		}
	})

	t.Run("valid", func(t *testing.T) {
		provider, err := Load(nil, dir, false, []v1alpha1.TestAttestation{{
			Image:         "ghcr.io/example/app:1.0",
			PredicateType: "https://cyclonedx.org/schema/bom",
			PredicateFile: "sbom.json",
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		statements, ok := provider.Get("ghcr.io/example/app:1.0", "https://cyclonedx.org/schema/bom")
		if !ok || len(statements) != 1 {
			t.Fatalf("expected one statement, got %v (ok=%v)", statements, ok)
		}
		predicate, ok := statements[0]["predicate"].(map[string]any)
		if !ok || predicate["bomFormat"] != "CycloneDX" {
			t.Fatalf("unexpected predicate: %v", statements[0])
		}
	})

	cases := []struct {
		name        string
		file        string
		errContains string
	}{
		{name: "missing file", file: "missing.json", errContains: "failed to read predicate file"},
		{name: "empty file", file: "empty.json", errContains: "is empty"},
		{name: "invalid json", file: "invalid.json", errContains: "invalid JSON"},
		{name: "empty object", file: "emptyobj.json", errContains: "empty JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(nil, dir, false, []v1alpha1.TestAttestation{{
				Image:         "ghcr.io/example/app:1.0",
				PredicateType: "https://cyclonedx.org/schema/bom",
				PredicateFile: tc.file,
			}})
			if err == nil || !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got %v", tc.errContains, err)
			}
		})
	}
}
