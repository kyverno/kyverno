package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ResolveGCEResourceFiles loads YAML files referenced by resourceFiles entries,
// converting them to RawExtensions in the Resources field.
func ResolveGCEResourceFiles(
	fs billy.Filesystem,
	testDir string,
	entries []v1alpha1.GlobalContextEntryValue,
) ([]v1alpha1.GlobalContextEntryValue, error) {
	result := make([]v1alpha1.GlobalContextEntryValue, len(entries))
	copy(result, entries)
	for i, entry := range result {
		if len(entry.ResourceFiles) == 0 {
			continue
		}
		var allResources []runtime.RawExtension
		for _, filePath := range entry.ResourceFiles {
			resources, err := loadResourceFile(fs, testDir, filePath)
			if err != nil {
				return nil, fmt.Errorf("globalContextEntries %q resourceFiles %q: %w", entry.Name, filePath, err)
			}
			allResources = append(allResources, resources...)
		}
		result[i].Resources = allResources
		result[i].ResourceFiles = nil
	}
	return result, nil
}

// loadResourceFile reads a YAML file, splits multi-doc, and returns []RawExtension.
func loadResourceFile(fs billy.Filesystem, testDir string, filePath string) ([]runtime.RawExtension, error) {
	fullPath := filepath.Join(testDir, filePath)

	var data []byte
	if fs != nil {
		file, err := fs.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("open: %w", err)
		}
		defer file.Close()
		data, err = io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
	} else {
		var err error
		data, err = os.ReadFile(fullPath) // #nosec G304
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
	}

	documents, err := extyaml.SplitDocuments(data)
	if err != nil {
		return nil, fmt.Errorf("split YAML documents: %w", err)
	}

	var resources []runtime.RawExtension
	for j, doc := range documents {
		var obj map[string]interface{}
		if err := yaml.UnmarshalStrict(doc, &obj); err != nil {
			return nil, fmt.Errorf("document[%d]: invalid YAML: %w", j, err)
		}
		if len(obj) == 0 {
			continue // skip empty documents
		}
		raw, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("document[%d]: JSON marshal: %w", j, err)
		}
		resources = append(resources, runtime.RawExtension{Raw: raw})
	}
	return resources, nil
}
