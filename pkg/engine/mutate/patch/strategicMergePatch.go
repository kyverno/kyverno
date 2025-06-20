package patch

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/kustomize/api/filters/patchstrategicmerge"
	filtersutil "sigs.k8s.io/kustomize/kyaml/filtersutil"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// ProcessStrategicMergePatch ...
func ProcessStrategicMergePatch(logger logr.Logger, overlay interface{}, resource resource) (resource, error) {
	overlayBytes, err := json.Marshal(overlay)
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return nil, err
	}
	patchedBytes, err := strategicMergePatch(logger, string(resource), string(overlayBytes))
	if err != nil {
		logger.Error(err, "failed to apply patchStrategicMerge")
		return nil, err
	}
	return patchedBytes, nil
}

func strategicMergePatch(logger logr.Logger, base, overlay string) ([]byte, error) {
	preprocessedYaml, err := preProcessStrategicMergePatch(logger, overlay, base)
	if err != nil {
		_, isConditionError := err.(ConditionError)
		_, isGlobalConditionError := err.(GlobalConditionError)

		if isConditionError || isGlobalConditionError {
			if err = preprocessedYaml.UnmarshalJSON([]byte(`{}`)); err != nil {
				return []byte{}, err
			}
		} else {
			return []byte{}, fmt.Errorf("failed to preProcess rule: %+v", err)
		}
	}

	patchStr, _ := preprocessedYaml.String()
	logger.V(3).Info("applying strategic merge patch", "patch", patchStr)
	f := patchstrategicmerge.Filter{
		Patch: preprocessedYaml,
	}

	baseObj := buffer{Buffer: bytes.NewBufferString(base)}
	err = filtersutil.ApplyToJSON(f, baseObj)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to apply JSON patch: %w", err)
	}
	patched, err := reorderContainers([]byte(base), baseObj.Bytes())
	if err != nil {
		return baseObj.Bytes(), nil
	}

	return patched, err
}

func preProcessStrategicMergePatch(logger logr.Logger, pattern, resource string) (*yaml.RNode, error) {
	patternNode := yaml.MustParse(pattern)
	resourceNode := yaml.MustParse(resource)

	err := PreProcessPattern(logger, patternNode, resourceNode)

	return patternNode, err
}

func reorderContainers(base, patched []byte) ([]byte, error) {
	var baseObj, patchedObj map[string]interface{}
	if err := json.Unmarshal(base, &baseObj); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(patched, &patchedObj); err != nil {
		return nil, err
	}
	for _, field := range []string{"containers", "initContainers"} {
		err := fixContainerListOrder(baseObj, patchedObj, field)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(patchedObj)
}

func fixContainerListOrder(baseObj, patchedObj map[string]interface{}, field string) error {
	specBase, ok1 := baseObj["spec"].(map[string]interface{})
	specPatched, ok2 := patchedObj["spec"].(map[string]interface{})
	if !ok1 || !ok2 {
		return nil
	}

	baseList, ok1 := specBase[field].([]interface{})
	patchedList, ok2 := specPatched[field].([]interface{})
	if !ok1 || !ok2 {
		return nil
	}

	m := make(map[string]interface{})
	for _, item := range patchedList {
		if c, ok := item.(map[string]interface{}); ok {
			var name string
			if v, has := c["name"]; has {
				name = fmt.Sprintf("%v", v)
				m[name] = c
			}
		}
	}

	var reordered []interface{}
	seen := make(map[string]bool)

	for _, item := range baseList {
		if c, ok := item.(map[string]interface{}); ok {
			var name string
			if v, has := c["name"]; has {
				name = fmt.Sprintf("%v", v)
				if match, exists := m[name]; exists {
					reordered = append(reordered, match)
					seen[name] = true
				}
			}
		}
	}

	for _, item := range patchedList {
		if c, ok := item.(map[string]interface{}); ok {
			var name string
			if v, has := c["name"]; has {
				name = fmt.Sprintf("%v", v)
				if !seen[name] {
					reordered = append(reordered, c)
					seen[name] = true
				}
			}
		}
	}

	specPatched[field] = reordered
	return nil
}
