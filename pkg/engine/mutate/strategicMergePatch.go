package mutate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
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
	resp.Type = utils.Mutation.String()

	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		resp.RuleStats.RuleExecutionTimestamp = startTime.Unix()
		logger.V(4).Info("finished applying strategicMerge patch", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	// ====== Meet Conditions =======
	if path, overlayerr := meetConditions(log, resource.UnstructuredContent(), overlay); !reflect.DeepEqual(overlayerr, overlayError{}) {
		switch overlayerr.statusCode {
		// anchor key does not exist in the resource, skip applying policy
		case conditionNotPresent:
			log.V(4).Info("skip applying policy", "path", path, "error", overlayerr)
			log.V(3).Info("skip applying rule", "reason", "conditionNotPresent")
			resp.Success = true
			return resp, resource
		// anchor key is not satisfied in the resource, skip applying policy
		case conditionFailure:
			log.V(4).Info("failed to validate condition", "path", path, "error", overlayerr)
			log.V(3).Info("skip applying rule", "reason", "conditionFailure")
			resp.Success = true
			resp.Message = overlayerr.ErrorMsg()
			return resp, resource
		}
	}
	// ============================

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
		log.Error(err, "failed to apply patchStrategicMerge")
		msg := fmt.Sprintf("failed to apply patchStrategicMerge: %v", err)
		resp.Success = false
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

	for _, p := range jsonPatches {
		log.V(5).Info("generated patch", "patch", string(p))
	}

	resp.Success = true
	resp.Patches = jsonPatches
	resp.Message = fmt.Sprintf("successfully processed strategic merge patch")
	return resp, patchedResource
}

func strategicMergePatch(base, overlay string) ([]byte, error) {

	preprocessedYaml, err := preProcessStrategicMergePatch(overlay, base)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to preProcess rule: %+v", err)
	}

	f := patchstrategicmerge.Filter{
		Patch: preprocessedYaml,
	}

	baseObj := buffer{Buffer: bytes.NewBufferString(base)}
	err = filtersutil.ApplyToJSON(f, baseObj)

	return baseObj.Bytes(), err
}

func preProcessStrategicMergePatch(pattern, resource string) (*yaml.RNode, error) {
	patternNode := yaml.MustParse(pattern)
	resourceNode := yaml.MustParse(resource)
	err := preProcessPattern(patternNode, resourceNode)
	return patternNode, err
}
