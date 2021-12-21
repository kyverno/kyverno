package utils

import (
	"fmt"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	commonAnchor "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//RuleType defines the type for rule
type RuleType int

const (
	//Mutation type for mutation rule
	Mutation RuleType = iota
	//Validation type for validation rule
	Validation
	//Generation type for generation rule
	Generation
	// ImageVerify type for image verification
	ImageVerify
)

func (ri RuleType) String() string {
	return [...]string{
		"Mutation",
		"Validation",
		"Generation",
		"All",
	}[ri]
}

// ApplyPatches patches given resource with given patches and returns patched document
// return original resource if any error occurs
func ApplyPatches(resource []byte, patches [][]byte) ([]byte, error) {
	joinedPatches := JoinPatches(patches)
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

// JoinPatches joins array of serialized JSON patches to the single JSONPatch array
func JoinPatches(patches [][]byte) []byte {
	var result []byte
	if len(patches) == 0 {
		return result
	}

	result = append(result, []byte("[\n")...)
	for index, patch := range patches {
		result = append(result, patch...)
		if index != len(patches)-1 {
			result = append(result, []byte(",\n")...)
		}
	}

	result = append(result, []byte("\n]")...)
	return result
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
