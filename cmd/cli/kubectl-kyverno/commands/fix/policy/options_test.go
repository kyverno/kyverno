package policy

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestProcessFileSavesFile verifies that when a policy uses the deprecated
// match.resources syntax (which fix.FixPolicy migrates to match.any), the
// file is rewritten with the hardened 0600 permission when save is enabled.
func TestProcessFileSavesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "policy.yaml")
	content := `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: cp
spec:
  rules:
  - name: r
    match:
      resources:
        kinds:
        - Pod
    exclude:
      resources:
        kinds:
        - Pod
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	o := options{save: true}
	var buf bytes.Buffer
	o.processFile(&buf, p)
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("expected file to exist after save: %v", err)
	}
	// owner read/write only (0600)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected file mode 0600, got %o", info.Mode().Perm())
	}
}
