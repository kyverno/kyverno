package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProcessOverlay handles validating admission request
// Checks the target resourse for rules defined in the policy
func ProcessOverlay(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([]PatchBytes, []byte) {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil || rule.Mutation.Overlay == nil {
			continue
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			log.Printf("Rule \"%s\" is not applicable to resource\n", rule.Name)
			continue
		}

		overlay := *rule.Mutation.Overlay
		if err, _ := applyOverlay(resource, overlay, ""); err != nil {
			//return fmt.Errorf("%s: %s", *rule.Validation.Message, err.Error())
		}
	}

	return nil, nil
}

func applyOverlay(resource, overlay interface{}, path string) ([]PatchBytes, error) {
	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})

		for key, value := range typedOverlay {
			path += "/" + key
			resourcePart, ok := typedResource[key]

			if ok {
				applyOverlay(resourcePart, value, path)
			} else {
				createSubtree(value, path)
			}
		}
	case []interface{}:
		typedResource := resource.([]interface{})
		applyOverlayToArray(typedResource, typedOverlay, path)
	case string:
		typedResource, ok := resource.(string)
		if !ok {
			return nil, fmt.Errorf("Expected string, found %T", resource)
		}
		replaceResource(typedResource, typedOverlay, path)
	case float64:
		typedResource, ok := resource.(float64)
		if !ok {
			return nil, fmt.Errorf("Expected string, found %T", resource)
		}
		replaceResource(typedResource, typedOverlay, path)
	case int64:
		typedResource, ok := resource.(int64)
		if !ok {
			return nil, fmt.Errorf("Expected string, found %T", resource)
		}
		replaceResource(typedResource, typedOverlay, path)
	}

	return nil, nil
}

func applyOverlayToArray(resource, overlay []interface{}, path string) {
	switch overlay[0].(type) {
	case map[string]interface{}:
		for _, overlayElement := range overlay {
			typedOverlay := overlayElement.(map[string]interface{})

			anchors := GetAnchorsFromMap(typedOverlay)

			if len(anchors) > 0 {
				// Try to replace
				for i, resourceElement := range resource {
					path += "/" + strconv.Itoa(i)
					typedResource := resourceElement.(map[string]interface{})
					if !skipArrayObject(typedResource, anchors) {
						replaceResource(typedResource, typedOverlay, path)
					}
				}
			} else {
				// Add new item to the front
				path += "/0"
				createSubtree(typedOverlay, path)
			}
		}
	default:
		path += "/0"
		for _, value := range overlay {
			createSubtree(value, path)
		}
	}
}

func skipArrayObject(object, anchors map[string]interface{}) bool {
	for key, pattern := range anchors {
		key = key[1 : len(key)-1]

		value, ok := object[key]
		if !ok {
			return true
		}

		return value != pattern
	}

	return false
}

func replaceResource(resource, overlay interface{}, path string) {

}

func createSubtree(overlayPart interface{}, path string) []PatchBytes {

	return nil
}
