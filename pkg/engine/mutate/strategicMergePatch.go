package mutate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/filters/patchstrategicmerge"
	filtersutil "sigs.k8s.io/kustomize/kyaml/filtersutil"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

func ProcessStrategicMergePatch(ruleName string, overlay interface{}, resource unstructured.Unstructured, log logr.Logger) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	logger := log.WithName("ProcessStrategicMergePatch").WithValues("rule", ruleName)
	logger.V(4).Info("started applying strategicMerge patch", "startTime", startTime)
	resp.Name = ruleName
	resp.Type = utils.Mutation.String()

	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		logger.V(4).Info("finished applying strategicMerge patch", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	overlayBytes, err := json.Marshal(overlay)
	if err != nil {
		resp.Success = false
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to process patchStrategicMerge: %v", err)
		return resp, resource
	}

	base, err := json.Marshal(resource.Object)
	if err != nil {
		resp.Success = false
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to process patchStrategicMerge: %v", err)
		return resp, resource
	}

	patchedBytes, err := strategicMergePatch(string(base), string(overlayBytes))
	if err != nil {
		msg := fmt.Sprintf("failed to apply patchStrategicMerge: %v", err)
		resp.Success = false
		log.Info(msg)
		resp.Message = msg
		return resp, resource
	}

	err = patchedResource.UnmarshalJSON(patchedBytes)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to process patchStrategicMerge: %v", err)
		return resp, resource
	}

	log.V(6).Info("generating JSON patches from patched resource", "patchedResource", patchedResource.Object)

	jsonPatches, err := generatePatches(base, patchedBytes)
	if err != nil {
		msg := fmt.Sprintf("failed to generated JSON patches from patched resource: %v", err.Error())
		resp.Success = false
		log.Info(msg)
		resp.Message = msg
		return resp, patchedResource
	}

	resp.Success = true
	resp.Patches = jsonPatches
	resp.Message = fmt.Sprintf("successfully processed stragetic merge patch")
	return resp, patchedResource
}

func strategicMergePatch(base, overlay string) ([]byte, error) {
	patch := yaml.MustParse(overlay)

	f := patchstrategicmerge.Filter{
		Patch: patch,
	}

	baseObj := buffer{Buffer: bytes.NewBufferString(base)}
	err := filtersutil.ApplyToJSON(f, baseObj)

	return baseObj.Bytes(), err
}
