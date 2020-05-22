package utils

import (
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	//All type for other rule operations(future)
	All
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
		return resource, err
	}

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		return resource, err
	}

	return patchedDocument, err
}

//ApplyPatchNew patches given resource with given joined patches
func ApplyPatchNew(resource, patch []byte) ([]byte, error) {
	jsonpatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return nil, err
	}

	patchedResource, err := jsonpatch.Apply(resource)
	if err != nil {
		return nil, err
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
		if anchor.IsConditionAnchor(key) {
			result[key] = value
		}
	}

	return result
}
