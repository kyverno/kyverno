package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/filters/patchstrategicmerge"
	filtersutil "sigs.k8s.io/kustomize/kyaml/filtersutil"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// ProcessStrategicMergePatch ...
func ProcessStrategicMergePatch(ruleName string, overlay interface{}, resource unstructured.Unstructured, log logr.Logger) (engineapi.RuleResponse, unstructured.Unstructured) {
	startTime := time.Now()
	logger := log.WithName("ProcessStrategicMergePatch").WithValues("rule", ruleName)
	logger.V(4).Info("started applying strategicMerge patch", "startTime", startTime)

	defer func() {
		logger.V(4).Info("finished applying strategicMerge patch", "processingTime", time.Since(startTime))
	}()

	overlayBytes, err := json.Marshal(overlay)
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return *engineapi.RuleFail(ruleName, engineapi.Mutation, fmt.Sprintf("failed to process patchStrategicMerge: %v", err)), resource
	}

	base, err := json.Marshal(resource.Object)
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return *engineapi.RuleFail(ruleName, engineapi.Mutation, fmt.Sprintf("failed to process patchStrategicMerge: %v", err)), resource
	}
	patchedBytes, err := strategicMergePatch(logger, string(base), string(overlayBytes))
	if err != nil {
		log.Error(err, "failed to apply patchStrategicMerge")
		msg := fmt.Sprintf("failed to apply patchStrategicMerge: %v", err)
		return *engineapi.RuleFail(ruleName, engineapi.Mutation, msg), resource
	}

	var patchedResource unstructured.Unstructured
	err = patchedResource.UnmarshalJSON(patchedBytes)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		return *engineapi.RuleFail(ruleName, engineapi.Mutation, fmt.Sprintf("failed to process patchStrategicMerge: %v", err)), resource
	}

	log.V(6).Info("generating JSON patches from patched resource", "patchedResource", patchedResource.Object)

	jsonPatches, err := generatePatches(base, patchedBytes)
	if err != nil {
		msg := fmt.Sprintf("failed to generated JSON patches from patched resource: %v", err.Error())
		log.V(2).Info(msg)
		return *engineapi.RuleFail(ruleName, engineapi.Mutation, msg), patchedResource
	}

	for _, p := range jsonPatches {
		log.V(5).Info("generated patch", "patch", string(p))
	}

	return *engineapi.RulePass(ruleName, engineapi.Mutation, "applied strategic merge patch").WithPatches(jsonPatches...), patchedResource
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
