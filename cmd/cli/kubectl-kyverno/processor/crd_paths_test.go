package processor

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

const widgetCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`

// A YAML file that does not parse, sitting next to a --crd-paths file, used to
// crash kubectl-validate while it looked for CRDs in the whole directory (#17727).
func TestCrdFileSystems_IgnoresUnlistedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	crdPath := filepath.Join(dir, "widget-crd.yaml")
	assert.NilError(t, os.WriteFile(crdPath, []byte(widgetCRD), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "notes.yaml"), []byte("key: [unclosed\n"), 0o600))

	fileSystems := crdFileSystems([]string{crdPath})
	assert.Equal(t, len(fileSystems), 1)
	paths, err := openapiclient.NewLocalCRDFiles(fileSystems...).Paths()
	assert.NilError(t, err)
	_, ok := paths["apis/example.com/v1"]
	assert.Assert(t, ok, "widget CRD not found, got %v", paths)
}

func TestCrdFileSystems_GroupsFilesByDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	for _, path := range []string{first, second, filepath.Join(dir, "other.yaml")} {
		assert.NilError(t, os.WriteFile(path, []byte(widgetCRD), 0o600))
	}

	fileSystems := crdFileSystems([]string{first, "", second})
	assert.Equal(t, len(fileSystems), 1)
	entries, err := fileSystems[0].(filteredDirFS).ReadDir(".")
	assert.NilError(t, err)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.DeepEqual(t, names, []string{"first.yaml", "second.yaml"})

	// a directory listed as a whole keeps all of its files
	fileSystems = crdFileSystems([]string{first, dir})
	assert.Equal(t, len(fileSystems), 1)
	_, filtered := fileSystems[0].(filteredDirFS)
	assert.Assert(t, !filtered)
}
