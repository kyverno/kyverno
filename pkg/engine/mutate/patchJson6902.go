package mutate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	patchjson6902 "sigs.k8s.io/kustomize/api/filters/patchjson6902"
	filtersutil "sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/yaml"
)

func ProcessPatchJSON6902(ruleName string, mutation kyverno.Mutation, resource unstructured.Unstructured, log logr.Logger) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	logger := log.WithValues("rule", ruleName)
	startTime := time.Now()
	logger.V(4).Info("started JSON6902 patch", "startTime", startTime)
	resp.Name = ruleName
	resp.Type = utils.Mutation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		logger.V(4).Info("applied JSON6902 patch", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		resp.Success = false
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to marshal resource: %v", err)
		return resp, resource
	}

	patchedResourceRaw, err := patchJSON6902(string(resourceRaw), mutation.PatchesJSON6902)
	if err != nil {
		resp.Success = false
		logger.V(3).Info("failed to process JSON6902 patches", "error", err.Error())
		resp.Message = fmt.Sprintf("failed to process JSON6902 patches: %v", err)
		return resp, resource
	}

	err = patchedResource.UnmarshalJSON(patchedResourceRaw)
	if err != nil {
		logger.Error(err, "failed to unmmarshal resource")
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to unmmarshal resource: %v", err)
		return resp, resource
	}

	var op []byte
	if mutation.PatchesJSON6902[0] != '[' {
		// if it doesn't seem to be JSON, imagine
		// it is YAML, and convert to JSON.
		op, err = yaml.YAMLToJSON([]byte(mutation.PatchesJSON6902))
		if err != nil {
			resp.Success = false
			resp.Message = fmt.Sprintf("failed to unmmarshal resource: %v", err)
			return resp, resource
		}
		mutation.PatchesJSON6902 = string(op)
	}

	var decodedPatch []kyverno.Patch
	err = json.Unmarshal(op, &decodedPatch)
	if err != nil {
		resp.Success = false
		resp.Message = err.Error()
		return resp, resource
	}

	patchesBytes, err := utils.TransformPatches(decodedPatch)
	if err != nil {
		logger.Error(err, "failed to marshal patches to bytes array")
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to marshal patches to bytes array: %v", err)
		return resp, resource
	}

	for _, p := range patchesBytes {
		log.V(6).Info("", "patches", string(p))
	}

	// JSON patches processed successfully
	resp.Success = true
	resp.Message = fmt.Sprintf("successfully process JSON6902 patches")
	resp.Patches = patchesBytes
	return resp, patchedResource
}

func patchJSON6902(base, patches string) ([]byte, error) {
	f := patchjson6902.Filter{
		Patch: patches,
	}

	baseObj := buffer{Buffer: bytes.NewBufferString(base)}
	err := filtersutil.ApplyToJSON(f, baseObj)

	return baseObj.Bytes(), err
}

func decodePatch(patch string) (jsonpatch.Patch, error) {
	// If the patch doesn't look like a JSON6902 patch, we
	// try to parse it to json.
	if !strings.HasPrefix(patch, "[") {
		p, err := yaml.YAMLToJSON([]byte(patch))
		if err != nil {
			return nil, err
		}
		patch = string(p)
	}

	decodedPatch, err := jsonpatch.DecodePatch([]byte(patch))
	if err != nil {
		return nil, err
	}

	return decodedPatch, nil
}
