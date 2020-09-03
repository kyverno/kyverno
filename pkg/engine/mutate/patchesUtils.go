package mutate

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/mattbaird/jsonpatch"

	evanjsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func generatePatches(src, dst []byte) ([][]byte, error) {
	var patchesBytes [][]byte
	pp, err := jsonpatch.CreatePatch(src, dst)
	for _, p := range pp {
		// TODO: handle remove nil value, i.e.,
		// {remove /spec/securityContext <nil>}
		// {remove /status/conditions/0/lastProbeTime <nil>}

		pbytes, err := p.MarshalJSON()
		if err != nil {
			return patchesBytes, err
		}

		patchesBytes = append(patchesBytes, pbytes)
		fmt.Printf("generated patch %s\n", p)
	}

	return patchesBytes, err
}

// preProcessJSONPatchesgo deals with the JsonPatch when reinvocation
// policy is set in webhook, to avoid generating duplicate values.
// This duplicate error only occurs on type array, if it's adding to a map
// the value will be added to the map if nil, otherwise it overwrites the old value
// return skip == true to skip the json patch application
func preProcessJSONPatches(mutation kyverno.Mutation, resource unstructured.Unstructured,
	log logr.Logger) (skip bool, err error) {
	var patches evanjsonpatch.Patch
	log = log.WithName("preProcessJSONPatches")

	if len(mutation.PatchesJSON6902) > 0 {
		patches, err = decodePatch(mutation.PatchesJSON6902)
		if err != nil {
			return false, fmt.Errorf("failed to process JSON patches: %v", err)
		}
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
			log.V(4).Info("failed to get object by the given path", "path", path, "error", err.Error())
			continue
		}

		val, err := patch.ValueInterface()
		if err != nil {
			log.V(4).Info("failed to get value by the given path", "path", path, "error", err.Error())
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
