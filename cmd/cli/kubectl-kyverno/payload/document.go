package payload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	decoder := json.NewDecoder(bytes.NewReader(d.Raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid JSON document %s: %w", d.Name, err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON document %s requires an object root for validation, image validation or deletion policies; use an object-producing mutation or omit those policies", d.Name)
	}
	if _, err := normalizeNumbers(object); err != nil {
		return nil, fmt.Errorf("invalid JSON document %s: %w", d.Name, err)
	}
	return &unstructured.Unstructured{Object: object}, nil
}

// normalizeNumbers converts json.Number values into the numeric types that
// unstructured and CEL support. Integer literals that fit in int64 stay exact
// (matching Kubernetes resources and JSON mutation) instead of being rounded
// through float64; other numbers keep the default float64 representation.
func normalizeNumbers(value any) (any, error) {
	var err error
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if typed[key], err = normalizeNumbers(item); err != nil {
				return nil, err
			}
		}
		return typed, nil
	case []any:
		for i, item := range typed {
			if typed[i], err = normalizeNumbers(item); err != nil {
				return nil, err
			}
		}
		return typed, nil
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return i, nil
		}
		if !strings.ContainsAny(typed.String(), ".eE") {
			return nil, fmt.Errorf("integer %s is outside the int64 range", typed)
		}
		f, err := typed.Float64()
		if err != nil {
			return nil, fmt.Errorf("number %s is out of range", typed)
		}
		return f, nil
	default:
		return value, nil
	}
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
