package policy

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// captureStderr runs f while capturing everything written to os.Stderr.
func captureStderr(f func()) string {
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return ""
	}
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// TestDeprecatedFieldWarning asserts that deprecated spec fields (e.g.
// spec.validationFailureAction, spec.webhookTimeoutSeconds) are surfaced by the
// CLI loader, not only by the admission webhook. This is the offline part of
// the migration path (plan Step 2) that previously was only enforced in-cluster.
func TestDeprecatedFieldWarning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := `apiVersion: kyverno.io/v2beta1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: Enforce
  webhookTimeoutSeconds: 15
  rules:
  - name: check-team
    match:
      any:
      - resources:
          kinds: [Pod]
    validate:
      message: "label 'team' is required"
      pattern:
        metadata:
          labels:
            team: "?*"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	WarningsAsErrors = false
	out := captureStderr(func() {
		if _, err := Load(nil, "", path); err != nil {
			t.Fatalf("unexpected error loading policy with deprecated fields: %v", err)
		}
	})
	if !strings.Contains(out, "spec.validationFailureAction is deprecated") {
		t.Fatalf("expected field deprecation warning for validationFailureAction, got:\n%s", out)
	}
	if !strings.Contains(out, "spec.webhookTimeoutSeconds is deprecated") {
		t.Fatalf("expected field deprecation warning for webhookTimeoutSeconds, got:\n%s", out)
	}

	WarningsAsErrors = true
	defer func() { WarningsAsErrors = false }()
	if _, err := Load(nil, "", path); err == nil {
		t.Fatal("expected deprecation error with --warnings-as-errors")
	} else if !errors.Is(err, deprecations.ErrDeprecated) {
		t.Fatalf("expected ErrDeprecated, got %v", err)
	}
}
