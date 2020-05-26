package policy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPolicy applies policy on a resource
//TODO: generation rules
func applyPolicy(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, logger logr.Logger) (responses []response.EngineResponse) {
	startTime := time.Now()
	defer func() {
		name := resource.GetKind() + "/" + resource.GetName()
		ns := resource.GetNamespace()
		if ns != "" {
			name = ns + "/" + name
		}

		logger.V(3).Info("applyPolicy", "resource", name, "processingTime", time.Since(startTime))
	}()

	var engineResponses []response.EngineResponse
	var engineResponseMutation, engineResponseValidation response.EngineResponse
	var err error
	// build context
	ctx := context.NewContext()
	err = ctx.AddResource(transformResource(resource))
	if err != nil {
		logger.Error(err, "enable to add transform resource to ctx")
	}
	//MUTATION
	engineResponseMutation, err = mutation(policy, resource, ctx, logger)
	if err != nil {
		logger.Error(err, "failed to process mutation rule")
	}

	//VALIDATION
	engineResponseValidation = engine.Validate(engine.PolicyContext{Policy: policy, Context: ctx, NewResource: resource})
	engineResponses = append(engineResponses, mergeRuleRespose(engineResponseMutation, engineResponseValidation))

	//TODO: GENERATION
	return engineResponses
}
func mutation(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, ctx context.EvalInterface, log logr.Logger) (response.EngineResponse, error) {

	engineResponse := engine.Mutate(engine.PolicyContext{Policy: policy, NewResource: resource, Context: ctx})
	if !engineResponse.IsSuccesful() {
		log.V(4).Info("failed to apply mutation rules; reporting them")
		return engineResponse, nil
	}
	// Verify if the JSON pathes returned by the Mutate are already applied to the resource
	if reflect.DeepEqual(resource, engineResponse.PatchedResource) {
		// resources matches
		log.V(4).Info("resource already satisfys the policy")
		return engineResponse, nil
	}
	return getFailedOverallRuleInfo(resource, engineResponse, log)
}

// getFailedOverallRuleInfo gets detailed info for over-all mutation failure
func getFailedOverallRuleInfo(resource unstructured.Unstructured, engineResponse response.EngineResponse, log logr.Logger) (response.EngineResponse, error) {
	rawResource, err := resource.MarshalJSON()
	if err != nil {
		log.Error(err, "faield to marshall resource")
		return response.EngineResponse{}, err
	}

	// resource does not match so there was a mutation rule violated
	for index, rule := range engineResponse.PolicyResponse.Rules {
		log.V(4).Info("veriying if policy rule was applied before", "rule", rule.Name)

		patches := dropKyvernoAnnotation(rule.Patches, log)
		if len(patches) == 0 {
			continue
		}

		patch, err := jsonpatch.DecodePatch(utils.JoinPatches(patches))
		if err != nil {
			log.Error(err, "failed to decode JSON patch", "patches", patches)
			return response.EngineResponse{}, err
		}

		// apply the patches returned by mutate to the original resource
		patchedResource, err := patch.Apply(rawResource)
		if err != nil {
			log.Error(err, "failed to apply JSON patch", "patches", patches)
			return response.EngineResponse{}, err
		}
		if !jsonpatch.Equal(patchedResource, rawResource) {
			log.V(4).Info("policy rule conditions not satisfied by resource", "rule", rule.Name)
			engineResponse.PolicyResponse.Rules[index].Success = false
			engineResponse.PolicyResponse.Rules[index].Message = fmt.Sprintf("mutation json patches not found at resource path %s", extractPatchPath(patches, log))
		}
	}

	return engineResponse, nil
}

type jsonPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func extractPatchPath(patches [][]byte, log logr.Logger) string {
	var resultPath []string
	// extract the patch path and value
	for _, patch := range patches {
		log.V(4).Info("expected json patch not found in resource", "patch", string(patch))
		var data jsonPatch
		if err := json.Unmarshal(patch, &data); err != nil {
			log.Error(err, "failed to decode the generate patch", "patch", string(patch))
			continue
		}
		resultPath = append(resultPath, data.Path)
	}
	return strings.Join(resultPath, ";")
}

func dropKyvernoAnnotation(patches [][]byte, log logr.Logger) (resultPathes [][]byte) {
	for _, patch := range patches {
		var data jsonPatch
		if err := json.Unmarshal(patch, &data); err != nil {
			log.Error(err, "failed to decode the generate patch", "patch", string(patch))
			continue
		}

		value := fmt.Sprintf("%v", data.Value)
		if strings.Contains(value, engine.PodTemplateAnnotation) {
			continue
		}

		resultPathes = append(resultPathes, patch)
	}
	return
}

func mergeRuleRespose(mutation, validation response.EngineResponse) response.EngineResponse {
	mutation.PolicyResponse.Rules = append(mutation.PolicyResponse.Rules, validation.PolicyResponse.Rules...)
	return mutation
}
