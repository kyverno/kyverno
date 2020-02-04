package mutate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPatch applies patch for resource, returns patched resource.
func applyPatch(resource []byte, patchRaw []byte) ([]byte, error) {
	patchesList := [][]byte{patchRaw}
	return utils.ApplyPatches(resource, patchesList)
}

//ProcessPatches applies the patches on the resource and returns the patched resource
func ProcessPatches(rule kyverno.Rule, resource unstructured.Unstructured) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	glog.V(4).Infof("started JSON patch rule %q (%v)", rule.Name, startTime)
	resp.Name = rule.Name
	resp.Type = utils.Mutation.String()
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

	// JSON patches processed successfully
	resp.Success = true
	resp.Message = fmt.Sprintf("successfully process JSON patches")
	resp.Patches = patches
	return resp, patchedResource
}
