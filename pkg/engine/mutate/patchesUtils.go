package mutate

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	evanjsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"github.com/mattbaird/jsonpatch"
	"github.com/minio/minio/pkg/wildcard"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func generatePatches(src, dst []byte) ([][]byte, error) {
	var patchesBytes [][]byte
	pp, err := jsonpatch.CreatePatch(src, dst)
	if err != nil {
		return nil, err
	}

	sortedPatches := filterAndSortPatches(pp)
	for _, p := range sortedPatches {
		pbytes, err := p.MarshalJSON()
		if err != nil {
			return patchesBytes, err
		}

		patchesBytes = append(patchesBytes, pbytes)
	}

	return patchesBytes, err
}

// filterAndSortPatches
// 1. filters out patches with the certain paths
// 2. sorts the removal patches(with same path) by the key of index
// in descending order. The sort is required as when removing multiple
// elements from an array, the elements must be removed in descending
// order to preserve each index value.
func filterAndSortPatches(originalPatches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	patches := filterInvalidPatches(originalPatches)

	result := make([]jsonpatch.JsonPatchOperation, len(patches))
	index := getIndexToBeReversed(patches)

	if len(index) == 0 {
		return patches
	}

	start := 0
	for _, idx := range index {
		end := idx[0]
		copy(result[start:end], patches[:end])
		reversedPatches := reverse(patches, idx)
		copy(result[end:], reversedPatches)
		start = idx[1] + 1
	}
	copy(result[start:], patches[start:])
	return result
}

func getIndexToBeReversed(patches []jsonpatch.JsonPatchOperation) [][]int {
	removePaths := make([]string, len(patches))

	for i, patch := range patches {
		if patch.Operation == "remove" {
			removePaths[i] = patch.Path
		}
	}
	return getRemoveInterval(removePaths)

}

func getRemoveInterval(removePaths []string) [][]int {
	// get paths end with '/number'
	regex := regexp.MustCompile(`\/\d+$`)
	for i, path := range removePaths {
		if !regex.Match([]byte(path)) {
			removePaths[i] = ""
		}
	}

	res := [][]int{}
	for i := 0; i < len(removePaths); {
		if removePaths[i] != "" {
			baseDir := filepath.Dir(removePaths[i])
			j := i + 1
			for ; j < len(removePaths); j++ {
				curDir := filepath.Dir(removePaths[j])
				if baseDir != curDir {
					break
				}
			}
			if i != j-1 {
				res = append(res, []int{i, j - 1})
			}
			i = j
		} else {
			i++
		}
	}

	return res
}

func reverse(patches []jsonpatch.JsonPatchOperation, interval []int) []jsonpatch.JsonPatchOperation {
	res := make([]jsonpatch.JsonPatchOperation, interval[1]-interval[0]+1)
	j := 0
	for i := interval[1]; i >= interval[0]; i-- {
		res[j] = patches[i]
		j++
	}
	return res
}

// filterInvalidPatches filters out patch with the following path:
// - not */metadata/name, */metadata/namespace, */metadata/labels, */metadata/annotations
// - /status
func filterInvalidPatches(patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	res := []jsonpatch.JsonPatchOperation{}
	for _, patch := range patches {
		if ignorePatch(patch.Path) {
			continue
		}

		res = append(res, patch)
	}
	return res
}

func ignorePatch(path string) bool {
	if strings.Contains(path, "/status") {
		return true
	}

	if wildcard.Match("*/metadata", path) {
		return false
	}

	if strings.Contains(path, "/metadata") {
		if !strings.Contains(path, "/metadata/name") &&
			!strings.Contains(path, "/metadata/namespace") &&
			!strings.Contains(path, "/metadata/annotations") &&
			!strings.Contains(path, "/metadata/labels") {
			return true
		}
	}

	return false
}

// preProcessJSONPatches deals with the JsonPatch when reinvocation
// policy is set in webhook, to avoid generating duplicate values.
// This duplicate error only occurs on type array, if it's adding to a map
// the value will be added to the map if nil, otherwise it overwrites the old value
// return skip == true to skip the json patch application
func preProcessJSONPatches(patchesJSON6902 []byte, resource unstructured.Unstructured,
	log logr.Logger) (skip bool, err error) {
	var patches evanjsonpatch.Patch
	log = log.WithName("preProcessJSONPatches")

	patches, err = evanjsonpatch.DecodePatch(patchesJSON6902)
	if err != nil {
		return false, fmt.Errorf("cannot decode patches as an RFC 6902 patch: %v", err)
	}

	for _, patch := range patches {
		if patch.Kind() != "add" {
			continue
		}

		path, err := patch.Path()
		if err != nil {
			return false, fmt.Errorf("failed to get path in JSON Patch: %v", err)
		}

		// check if the target is the list
		if tail := filepath.Base(path); tail != "-" {
			_, err := strconv.Atoi(tail)
			if err != nil {
				log.V(4).Info("JSON patch does not add to the list, skipping", "path", path)
				continue
			}
		}

		resourceObj, err := getObject(path, resource.UnstructuredContent())
		if err != nil {
			log.V(4).Info("unable to get object by the given path, proceed patchesJson6902 without preprocessing", "path", path, "error", err.Error())
			continue
		}

		val, err := patch.ValueInterface()
		if err != nil {
			log.V(4).Info("unable to get value by the given path, proceed patchesJson6902 without preprocessing", "path", path, "error", err.Error())
			continue
		}

		// if there's one patch exist in the resource, which indicates
		// this is re-invoked JSON patches, skip application
		if isSubsetObject(val, resourceObj) {
			return true, nil
		}
	}

	return false, nil
}

// - insert to the end of the list
// {"op": "add", "path": "/spec/containers/-", {"value": "{"name":"busyboxx","image":"busybox:latest"}"}

// - insert value to the certain element of the list
// {"op": "add", "path": "/spec/containers/1", {"value": "{"name":"busyboxx","image":"busybox:latest"}"}
func getObject(path string, resource map[string]interface{}) (interface{}, error) {
	var strippedResource interface{}
	strippedResource = resource
	var ok bool

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	paths := strings.Split(path, "/")

	for i, key := range paths {
		switch strippedResource.(type) {
		case map[string]interface{}:
			strippedResource, ok = strippedResource.(map[string]interface{})[key]
			if !ok {
				return nil, fmt.Errorf("referenced value does not exist at %s", strings.Join(paths[:i+1], "/"))
			}

		case []interface{}:
			var idx int

			if key == "-" {
				idx = len(strippedResource.([]interface{})) - 1
			} else {
				var err error
				idx, err = strconv.Atoi(key)
				if err != nil {
					return nil, fmt.Errorf("cannot parse index in JSON Patch at %s: %v", strings.Join(paths[:i+1], "/"), err)
				}
				if idx < 0 {
					idx = len(strippedResource.([]interface{})) - 1
				}
			}

			if len(strippedResource.([]interface{})) <= idx {
				return nil, nil
			}

			strippedResource = strippedResource.([]interface{})[idx]
		}
	}
	return strippedResource, nil
}

// isSubsetObject returns true if object is subset of resource
// the object/resource is the element inside the list, return false
// if the type is mismatched (not map)
func isSubsetObject(object, resource interface{}) bool {
	objectMap, ok := object.(map[string]interface{})
	if !ok {
		return false
	}

	resourceMap, ok := resource.(map[string]interface{})
	if !ok {
		return false
	}

	for objKey, objVal := range objectMap {
		rsrcVal, ok := resourceMap[objKey]
		if !ok {
			return false
		}

		if !reflect.DeepEqual(objVal, rsrcVal) {
			return false
		}
	}
	return true
}
