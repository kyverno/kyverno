package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	jsonpatch "github.com/evanphx/json-patch"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

// ProcessPatches Returns array from separate patches that can be applied to the document
// Returns error ONLY in case when creation of resource should be denied.
func processPatches(resourceUnstr unstructured.Unstructured, rule kyverno.Rule) (allPatches [][]byte, errs []error) {
	//TODO check if there is better solution
	resource, err := resourceUnstr.MarshalJSON()
	if err != nil {
		glog.V(4).Infof("unable to marshal resource : %v", err)
		errs = append(errs, fmt.Errorf("unable to marshal resource : %v", err))
		return nil, errs
	}

	if len(resource) == 0 {
		errs = append(errs, errors.New("Source document for patching is empty"))
		return nil, errs
	}
	if reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
		errs = append(errs, errors.New("No Mutation rules defined"))
		return nil, errs
	}
	patchedDocument := resource
	for _, patch := range rule.Mutation.Patches {
		patchRaw, err := json.Marshal(patch)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		patchedDocument, err = applyPatch(patchedDocument, patchRaw)
		// TODO: continue on error if one of the patches fails, will add the failure event in such case
		if patch.Operation == "remove" {
			glog.Info(err)
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		allPatches = append(allPatches, patchRaw)
	}
	return allPatches, errs
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

// applyPatch applies patch for resource, returns patched resource.
func applyPatch(resource []byte, patchRaw []byte) ([]byte, error) {
	patchesList := [][]byte{patchRaw}
	return ApplyPatches(resource, patchesList)
}

// ApplyPatches patches given resource with given patches and returns patched document
func ApplyPatches(resource []byte, patches [][]byte) ([]byte, error) {
	joinedPatches := JoinPatches(patches)
	patch, err := jsonpatch.DecodePatch(joinedPatches)
	if err != nil {
		return nil, err
	}

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		return resource, err
	}
	return patchedDocument, err
}
