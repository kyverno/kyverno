package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

func processPatches(rule kyverno.Rule, resource unstructured.Unstructured) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	glog.V(4).Infof("started JSON patch rule %q (%v)", rule.Name, startTime)
	resp.Name = rule.Name
	resp.Type = Mutation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished JSON patch rule %q (%v)", resp.Name, resp.RuleStats.ProcessingTime)
	}()

	// convert to RAW
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		resp.Success = false
		glog.Infof("unable to marshall resource: %v", err)
		resp.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return resp, resource
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
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to process JSON patches: %v", func() string {
			var str []string
			for _, err := range errs {
				str = append(str, err.Error())
			}
			return strings.Join(str, ";")
		}())
		return resp, resource
	}
	err = patchedResource.UnmarshalJSON(resourceRaw)
	if err != nil {
		glog.Infof("failed to unmarshall resource to undstructured: %v", err)
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return resp, resource
	}

	// JSON patches processed succesfully
	resp.Success = true
	resp.Message = fmt.Sprintf("succesfully process JSON patches")
	resp.Patches = patches
	return resp, patchedResource
}
