package payload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kyverno/pkg/ext/file"
	"go.yaml.in/yaml/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Document keeps JSON bytes intact until evaluation, including non-object roots
// and integer literals which cannot be represented exactly as float64.
type Document struct {
	Name string
	Raw  json.RawMessage
}

func ParseDocument(name string, content []byte) (*Document, error) {
	if file.IsYaml(name) {
		var value any
		decoder := yaml.NewDecoder(bytes.NewReader(content))
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("JSON payload %s requires exactly one YAML document", name)
		}
		var err error
		content, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	} else if !file.IsJson(name) {
		return nil, fmt.Errorf("unrecognized payload format, must be yaml or json (%s)", name)
	}
	if !json.Valid(content) {
		return nil, fmt.Errorf("invalid JSON document %s: expected exactly one JSON value", name)
	}
	return &Document{Name: name, Raw: bytes.Clone(content)}, nil
}

func LoadDocument(name string) (*Document, error) {
	content, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		return nil, err
	}
	return ParseDocument(name, content)
}

func (d *Document) Object() (*unstructured.Unstructured, error) {
	var object map[string]any
	if err := json.Unmarshal(d.Raw, &object); err != nil || object == nil {
		return nil, fmt.Errorf("JSON document %s requires an object root for validation, image validation or deletion policies; use an object-producing mutation or omit those policies", d.Name)
	}
	return &unstructured.Unstructured{Object: object}, nil
}

func Objects(documents []*Document) ([]*unstructured.Unstructured, error) {
	result := make([]*unstructured.Unstructured, 0, len(documents))
	for _, document := range documents {
		object, err := document.Object()
		if err != nil {
			return nil, err
		}
		result = append(result, object)
	}
	return result, nil
}
