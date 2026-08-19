package policy

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kyverno/kyverno/pkg/deprecations"
)

func repoRoot(t *testing.T) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// TestDeprecatedAPIVersionWarning asserts that loading a v2beta1 fixture
// succeeds normally but returns deprecations.ErrDeprecated when the
// --warnings-as-errors flag is enabled. These fixtures intentionally use the
// deprecated apiVersion and serve as warning-assertion tests.
func TestDeprecatedAPIVersionWarning(t *testing.T) {
	root := repoRoot(t)
	fixtures := []string{
		"clusterpolicy.yaml",
		"policy.yaml",
		"policyexception.yaml",
		"cleanuppolicy.yaml",
		"clustercleanuppolicy.yaml",
	}
	for _, name := range fixtures {
		name := name
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "test", "cli", "deprecated-versions", name)
			WarningsAsErrors = false
			if _, err := Load(nil, "", path); err != nil {
				t.Fatalf("unexpected error loading deprecated fixture %s: %v", name, err)
			}
			WarningsAsErrors = true
			defer func() { WarningsAsErrors = false }()
			if _, err := Load(nil, "", path); err == nil {
				t.Fatalf("expected deprecation error for %s", name)
			} else if !errors.Is(err, deprecations.ErrDeprecated) {
				t.Fatalf("expected ErrDeprecated for %s, got %v", name, err)
			}
		})
	}
}
