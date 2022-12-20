package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/filters/patchstrategicmerge"
	filtersutil "sigs.k8s.io/kustomize/kyaml/filtersutil"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// ProcessStrategicMergePatch ...
func ProcessStrategicMergePatch(ruleName string, overlay interface{}, resource unstructured.Unstructured, log logr.Logger) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	logger := log.WithName("ProcessStrategicMergePatch").WithValues("rule", ruleName)
	logger.V(4).Info("started applying strategicMerge patch", "startTime", startTime)
	resp.Name = ruleName
	resp.Type = response.Mutation

	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		resp.RuleStats.RuleExecutionTimestamp = startTime.Unix()
		logger.V(4).Info("finished applying strategicMerge patch", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	overlayBytes, err := json.Marshal(overlay)
	if err != nil {
		resp.Status = response.RuleStatusFail
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to process patchStrategicMerge: %v", err)
		return resp, resource
	}

	base, err := json.Marshal(resource.Object)
	if err != nil {
		resp.Status = response.RuleStatusFail
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to process patchStrategicMerge: %v", err)
		return resp, resource
	}
	patchedBytes, err := strategicMergePatch(logger, string(base), string(overlayBytes))
	if err != nil {
		log.Error(err, "failed to apply patchStrategicMerge")
		msg := fmt.Sprintf("failed to apply patchStrategicMerge: %v", err)
		resp.Status = response.RuleStatusFail
		resp.Message = msg
		return resp, resource
	}

	err = patchedResource.UnmarshalJSON(patchedBytes)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		resp.Status = response.RuleStatusFail
		resp.Message = fmt.Sprintf("failed to process patchStrategicMerge: %v", err)
		return resp, resource
	}

	log.V(6).Info("generating JSON patches from patched resource", "patchedResource", patchedResource.Object)

	jsonPatches, err := generatePatches(base, patchedBytes)
	if err != nil {
		msg := fmt.Sprintf("failed to generated JSON patches from patched resource: %v", err.Error())
		resp.Status = response.RuleStatusFail
		log.V(2).Info(msg)
		resp.Message = msg
		return resp, patchedResource
	}

	for _, p := range jsonPatches {
		log.V(5).Info("generated patch", "patch", string(p))
	}

	resp.Status = response.RuleStatusPass
	resp.Patches = jsonPatches
	resp.Message = "applied strategic merge patch"
	return resp, patchedResource
}

func strategicMergePatch(logger logr.Logger, base, overlay string) ([]byte, error) {
	preprocessedYaml, err := preProcessStrategicMergePatch(logger, overlay, base)
	if err != nil {
		_, isConditionError := err.(ConditionError)
		_, isGlobalConditionError := err.(GlobalConditionError)

		if isConditionError || isGlobalConditionError {
			if err = preprocessedYaml.UnmarshalJSON([]byte(`{}`)); err != nil {
				return []byte{}, err
			}
		} else {
			return []byte{}, fmt.Errorf("failed to preProcess rule: %+v", err)
		}
	}

	patchStr, _ := preprocessedYaml.String()
	logger.V(3).Info("applying strategic merge patch", "patch", patchStr)
	f := patchstrategicmerge.Filter{
		Patch: preprocessedYaml,
	}

	baseObj := buffer{Buffer: bytes.NewBufferString(base)}
	err = filtersutil.ApplyToJSON(f, baseObj)

	return baseObj.Bytes(), err
}

func preProcessStrategicMergePatch(logger logr.Logger, pattern, resource string) (*yaml.RNode, error) {
	patternNode := yaml.MustParse(pattern)
	resourceNode := yaml.MustParse(resource)

	err := preProcessPattern(logger, patternNode, resourceNode)

	return patternNode, err
}
