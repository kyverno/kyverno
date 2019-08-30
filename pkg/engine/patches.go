package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	jsonpatch "github.com/evanphx/json-patch"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

// ProcessPatches Returns array from separate patches that can be applied to the document
// Returns error ONLY in case when creation of resource should be denied.
// TODO: pass in the unstructured object in stead of raw byte?
func processPatches(rule kyverno.Rule, resource []byte) (allPatches [][]byte, errs []error) {
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
		patches := [][]byte{patchRaw}
		patchedDocument, err = ApplyPatches(patchedDocument, patches)
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

//ApplyPatchNew ...
func ApplyPatchNew(resource, patch []byte) ([]byte, error) {
	patchesList := [][]byte{patch}
	joinedPatches := JoinPatches(patchesList)
	jsonpatch, err := jsonpatch.DecodePatch(joinedPatches)
	if err != nil {
		return nil, err
	}

	patchedResource, err := jsonpatch.Apply(resource)
	if err != nil {
		return nil, err
	}
	return patchedResource, err

}

func processPatchesNew(rule kyverno.Rule, resource unstructured.Unstructured) (response RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	glog.V(4).Infof("started JSON patch rule %q (%v)", rule.Name, startTime)
	response.Name = rule.Name
	response.Type = Mutation.String()
	defer func() {
		response.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished JSON patch rule %q (%v)", response.Name, response.RuleStats.ProcessingTime)
	}()

	// convert to RAW
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		response.Success = false
		glog.Infof("unable to marshall resource: %v", err)
		response.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return response, resource
	}

	var errs []error
	var patches [][]byte
	for _, patch := range rule.Mutation.Patches {
		// JSON patch
		patchRaw, err := json.Marshal(patch)
		if err != nil {
			glog.V(4).Infof("failed to marshall JSON patch %v: %v", patch, err)
			errs = append(errs, err)
			continue
		}
		patchResource, err := applyPatch(resourceRaw, patchRaw)
		// TODO: continue on error if one of the patches fails, will add the failure event in such case
		if err != nil && patch.Operation == "remove" {
			glog.Info(err)
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		resourceRaw = patchResource
		patches = append(patches, patchRaw)
	}

	// error while processing JSON patches
	if len(errs) > 0 {
		response.Success = false
		response.Message = fmt.Sprintf("failed to process JSON patches: %v", func() string {
			var str []string
			for _, err := range errs {
				str = append(str, err.Error())
			}
			return strings.Join(str, ";")
		}())
		return response, resource
	}
	err = patchedResource.UnmarshalJSON(resourceRaw)
	if err != nil {
		glog.Infof("failed to unmarshall resource to undstructured: %v", err)
		response.Success = false
		response.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return response, resource
	}

	// JSON patches processed succesfully
	response.Success = true
	response.Message = fmt.Sprintf("succesfully process JSON patches")
	response.Patches = patches
	return response, patchedResource
}
