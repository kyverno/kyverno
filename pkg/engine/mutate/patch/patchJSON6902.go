package patch

import (
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// ProcessPatchJSON6902 ...
func ProcessPatchJSON6902(ruleName string, patchesJSON6902 []byte, resource unstructured.Unstructured, log logr.Logger) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	logger := log.WithValues("rule", ruleName)
	startTime := time.Now()
	logger.V(4).Info("started JSON6902 patch", "startTime", startTime)
	resp.Name = ruleName
	resp.Type = utils.Mutation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		resp.RuleStats.RuleExecutionTimestamp = startTime.Unix()
		logger.V(4).Info("applied JSON6902 patch", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		resp.Status = response.RuleStatusFail
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to marshal resource: %v", err)
		return resp, resource
	}

	patchedResourceRaw, err := applyPatchesWithOptions(resourceRaw, patchesJSON6902)
	if err != nil {
		resp.Status = response.RuleStatusFail
		logger.Error(err, "failed to apply JSON Patch")
		resp.Message = fmt.Sprintf("failed to apply JSON Patch: %v", err)
		return resp, resource
	}

	patchesBytes, err := generatePatches(resourceRaw, patchedResourceRaw)
	if err != nil {
		resp.Status = response.RuleStatusFail
		logger.Error(err, "unable generate patch bytes from base and patched document, apply patchesJSON6902 directly")
		resp.Message = fmt.Sprintf("unable generate patch bytes from base and patched document, apply patchesJSON6902 directly: %v", err)
		return resp, resource
	}

	for _, p := range patchesBytes {
		log.V(4).Info("generated JSON Patch (RFC 6902)", "patch", string(p))
	}

	err = patchedResource.UnmarshalJSON(patchedResourceRaw)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		resp.Status = response.RuleStatusFail
		resp.Message = fmt.Sprintf("failed to unmarshal resource: %v", err)
		return resp, resource
	}

	resp.Status = response.RuleStatusPass
	resp.Message = string("applied JSON Patch")
	resp.Patches = patchesBytes
	return resp, patchedResource
}

func applyPatchesWithOptions(resource, patch []byte) ([]byte, error) {
	patches, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return resource, fmt.Errorf("failed to decode patches: %v", err)
	}

	options := &jsonpatch.ApplyOptions{SupportNegativeIndices: true, AllowMissingPathOnRemove: true, EnsurePathExistsOnAdd: true}
	patchedResource, err := patches.ApplyWithOptions(resource, options)
	if err != nil {
		return resource, err
	}

	return patchedResource, nil
}

func ConvertPatchesToJSON(patchesJSON6902 string) ([]byte, error) {
	if len(patchesJSON6902) == 0 {
		return []byte(patchesJSON6902), nil
	}

	if patchesJSON6902[0] != '[' {
		// If the patch doesn't look like a JSON6902 patch, we
		// try to parse it to json.
		op, err := yaml.YAMLToJSON([]byte(patchesJSON6902))
		if err != nil {
			return nil, fmt.Errorf("failed to convert patchesJSON6902 to JSON: %v", err)
		}
		return op, nil
	}

	return []byte(patchesJSON6902), nil
}
