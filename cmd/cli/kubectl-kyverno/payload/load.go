package payload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kyverno/pkg/ext/file"
	yamlutils "github.com/kyverno/pkg/ext/yaml"
	"go.yaml.in/yaml/v3"
)

func Load(path string) (any, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var payload any
	switch {
	case file.IsJson(path):
		if err := json.Unmarshal(content, &payload); err != nil {
			return nil, err
		}
	case file.IsYaml(path):
		documents, err := yamlutils.SplitDocuments(content)
		if err != nil {
			return nil, err
		}
		var objects []any
		for _, document := range documents {
			var object map[string]any
			if err := yaml.Unmarshal(document, &object); err != nil {
				return nil, err
			}
			objects = append(objects, object)
		}
		if len(objects) == 1 {
			payload = objects[0]
		} else {
			payload = objects
		}
	default:
		return nil, fmt.Errorf("unrecognized payload format, must be yaml or json (%s)", path)
	}
	return payload, nil
}
