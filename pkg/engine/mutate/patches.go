package mutate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPatch applies patch for resource, returns patched resource.
func applyPatch(resource []byte, patchRaw []byte) ([]byte, error) {
	patchesList := [][]byte{patchRaw}
	return utils.ApplyPatches(resource, patchesList)
}

//ProcessPatches applies the patches on the resource and returns the patched resource
func ProcessPatches(log logr.Logger, ruleName string, mutation kyverno.Mutation, resource unstructured.Unstructured) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	logger := log.WithValues("rule", ruleName)
	startTime := time.Now()
	logger.V(4).Info("started JSON patch", "startTime", startTime)
	resp.Name = ruleName
	resp.Type = utils.Mutation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		resp.RuleStats.RuleExecutionTimestamp = startTime.Unix()
		logger.V(4).Info("applied JSON patch", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	// convert to RAW
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		resp.Status = response.RuleStatusFail
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return resp, resource
	}

	var errs []error
	var patches [][]byte
	for _, patch := range mutation.Patches {
		// JSON patch
		patchRaw, err := json.Marshal(patch)
		if err != nil {
			logger.Error(err, "failed to marshal JSON patch")
			errs = append(errs, err)
			continue
		}
		patchResource, err := applyPatch(resourceRaw, patchRaw)
		if err != nil && patch.Operation == "remove" {
			log.Error(err, "failed to process JSON path or patch is a 'remove' operation")
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
		resp.Status = response.RuleStatusFail
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
		logger.Error(err, "failed to unmarshal resource")
		resp.Status = response.RuleStatusFail
		resp.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return resp, resource
	}

	// JSON patches processed successfully
	resp.Status = response.RuleStatusPass
	resp.Message = string("successfully process JSON patches")
	resp.Patches = patches
	return resp, patchedResource
}
