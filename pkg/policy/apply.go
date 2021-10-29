package policy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPolicy applies policy on a resource
func applyPolicy(policy kyverno.ClusterPolicy, resource unstructured.Unstructured,
	logger logr.Logger, excludeGroupRole []string, resCache resourcecache.ResourceCache,
	client *client.Client, namespaceLabels map[string]string) (responses []*response.EngineResponse) {

	startTime := time.Now()
	defer func() {
		name := resource.GetKind() + "/" + resource.GetName()
		ns := resource.GetNamespace()
		if ns != "" {
			name = ns + "/" + name
		}

		logger.V(3).Info("applyPolicy", "resource", name, "processingTime", time.Since(startTime).String())
	}()

	var engineResponses []*response.EngineResponse
	var engineResponseMutation, engineResponseValidation *response.EngineResponse
	var err error

	ctx := context.NewContext()
	err = ctx.AddResource(transformResource(resource))
	if err != nil {
		logger.Error(err, "failed to add transform resource to ctx")
	}

	err = ctx.AddNamespace(resource.GetNamespace())
	if err != nil {
		logger.Error(err, "failed to add namespace to ctx")
	}

	if err := ctx.AddImageInfo(&resource); err != nil {
		logger.Error(err, "unable to add image info to variables context")
	}

	engineResponseMutation, err = mutation(policy, resource, logger, resCache, ctx, namespaceLabels)
	if err != nil {
		logger.Error(err, "failed to process mutation rule")
	}

	policyCtx := &engine.PolicyContext{
		Policy:           policy,
		NewResource:      resource,
		ExcludeGroupRole: excludeGroupRole,
		ResourceCache:    resCache,
		JSONContext:      ctx,
		Client:           client,
		NamespaceLabels:  namespaceLabels,
	}

	engineResponseValidation = engine.Validate(policyCtx)
	engineResponses = append(engineResponses, mergeRuleRespose(engineResponseMutation, engineResponseValidation))

	return engineResponses
}

func mutation(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, log logr.Logger, resCache resourcecache.ResourceCache, jsonContext *context.Context, namespaceLabels map[string]string) (*response.EngineResponse, error) {

	policyContext := &engine.PolicyContext{
		Policy:          policy,
		NewResource:     resource,
		ResourceCache:   resCache,
		JSONContext:     jsonContext,
		NamespaceLabels: namespaceLabels,
	}

	engineResponse := engine.Mutate(policyContext)
	if !engineResponse.IsSuccessful() {
		log.V(4).Info("failed to apply mutation rules; reporting them")
		return engineResponse, nil
	}
	// Verify if the JSON patches returned by the Mutate are already applied to the resource
	if reflect.DeepEqual(resource, engineResponse.PatchedResource) {
		// resources matches
		log.V(4).Info("resource already satisfies the policy")
		return engineResponse, nil
	}
	return getFailedOverallRuleInfo(resource, engineResponse, log)
}

// getFailedOverallRuleInfo gets detailed info for over-all mutation failure
func getFailedOverallRuleInfo(resource unstructured.Unstructured, engineResponse *response.EngineResponse, log logr.Logger) (*response.EngineResponse, error) {
	rawResource, err := resource.MarshalJSON()
	if err != nil {
		log.Error(err, "failed to marshall resource")
		return &response.EngineResponse{}, err
	}

	// resource does not match so there was a mutation rule violated
	for index, rule := range engineResponse.PolicyResponse.Rules {
		log.V(4).Info("verifying if policy rule was applied before", "rule", rule.Name)

		patches := rule.Patches

		patch, err := jsonpatch.DecodePatch(utils.JoinPatches(patches))
		if err != nil {
			log.Error(err, "failed to decode JSON patch", "patches", patches)
			return &response.EngineResponse{}, err
		}

		// apply the patches returned by mutate to the original resource
		patchedResource, err := patch.Apply(rawResource)
		if err != nil {
			log.Error(err, "failed to apply JSON patch", "patches", patches)
			return &response.EngineResponse{}, err
		}

		if !jsonpatch.Equal(patchedResource, rawResource) {
			log.V(4).Info("policy rule conditions not satisfied by resource", "rule", rule.Name)
			engineResponse.PolicyResponse.Rules[index].Status = response.RuleStatusFail
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

func mergeRuleRespose(mutation, validation *response.EngineResponse) *response.EngineResponse {
	mutation.PolicyResponse.Rules = append(mutation.PolicyResponse.Rules, validation.PolicyResponse.Rules...)
	return mutation
}
