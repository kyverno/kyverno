package utils

import (
	"fmt"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	commonAnchor "github.com/kyverno/kyverno/pkg/engine/anchor"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ApplyPatches patches given resource with given patches and returns patched document
// return original resource if any error occurs
func ApplyPatches(resource []byte, patches [][]byte) ([]byte, error) {
	if len(patches) == 0 {
		return resource, nil
	}
	joinedPatches := jsonutils.JoinPatches(patches...)
	patch, err := jsonpatch.DecodePatch(joinedPatches)
	if err != nil {
		log.Log.V(4).Info("failed to decode JSON patch", "patch", patch)
		return resource, err
	}

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		log.Log.V(4).Info("failed to apply JSON patch", "patch", patch)
		return resource, err
	}

	log.Log.V(4).Info("applied JSON patch", "patch", patch)
	return patchedDocument, err
}

//ApplyPatchNew patches given resource with given joined patches
func ApplyPatchNew(resource, patch []byte) ([]byte, error) {
	jsonpatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return resource, err
	}

	patchedResource, err := jsonpatch.Apply(resource)
	if err != nil {
		return resource, err
	}

	return patchedResource, err

}

//ConvertToUnstructured converts the resource to unstructured format
func ConvertToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

// GetAnchorsFromMap gets the conditional anchor map
func GetAnchorsFromMap(anchorsMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range anchorsMap {
		if commonAnchor.IsConditionAnchor(key) {
			result[key] = value
		}
	}

	return result
}

func JsonPointerToJMESPath(jsonPointer string) string {
	var sb strings.Builder
	tokens := strings.Split(jsonPointer, "/")
	i := 0
	for _, t := range tokens {
		if t == "" {
			continue
		}

		if _, err := strconv.Atoi(t); err == nil {
			sb.WriteString(fmt.Sprintf("[%s]", t))
			continue
		}

		if i > 0 {
			sb.WriteString(".")
		}

		sb.WriteString(t)
		i++
	}

	return sb.String()
}

func CombineErrors(errors []error) error {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0]
	}

	messages := make([]string, len(errors))
	for i := range errors {
		messages[i] = errors[i].Error()
	}

	msg := strings.Join(messages, "; ")
	return fmt.Errorf(msg)
}
