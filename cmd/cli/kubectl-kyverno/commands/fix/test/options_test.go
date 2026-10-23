package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestExecuteSavesFiles verifies that the fix command rewrites the test file,
// the user info file and the values file (all using the deprecated syntax that
// fix.Fix* migrates) with the hardened 0600 permission when save is enabled.
func TestExecuteSavesFiles(t *testing.T) {
	dir := t.TempDir()
	testYAML := `kind: Test
metadata:
  name: kyverno-test
policies:
- policy.yaml
resources:
- resources.yaml
results:
- kind: Deployment
  policy: p
  resources:
  - r
  result: pass
  rule: rule
userinfo: userinfo.yaml
variables: values.yaml
`
	userInfoYAML := `kind: UserInfo
`
	valuesYAML := `kind: Values
`
	files := map[string]string{
		"kyverno-test.yaml": testYAML,
		"userinfo.yaml":     userInfoYAML,
		"values.yaml":       valuesYAML,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	o := options{fileName: "kyverno-test.yaml", save: true}
	var buf bytes.Buffer
	if err := o.execute(&buf, dir); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	for _, f := range []string{"kyverno-test.yaml", "userinfo.yaml", "values.yaml"} {
		info, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("expected %s to exist after save: %v", f, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("expected %s mode 0600, got %o", f, info.Mode().Perm())
		}
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("failed to read %s: %v", f, err)
		}
		// the fix command should have added the apiVersion on save
		if !bytes.Contains(content, []byte("apiVersion: cli.kyverno.io/v1alpha1")) {
			t.Fatalf("expected %s to be rewritten by the fix command (missing apiVersion):\n%s", f, content)
		}
	}
}
