package policy

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPolicy applies policy on a resource
func applyPolicy(
	policy kyvernov1.PolicyInterface,
	resource unstructured.Unstructured,
	logger logr.Logger,
	excludeGroupRole []string,
	client dclient.Interface,
	rclient registryclient.Client,
	informerCacheResolvers resolvers.ConfigmapResolver,
	namespaceLabels map[string]string,
) (responses []*response.EngineResponse) {
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

	ctx := enginecontext.NewContext()
	data, err := resource.MarshalJSON()
	if err != nil {
		logging.Error(err, "failed to marshal resource")
	}
	err = enginecontext.AddResource(ctx, data)
	if err != nil {
		logger.Error(err, "failed to add transform resource to ctx")
	}
	err = ctx.AddNamespace(resource.GetNamespace())
	if err != nil {
		logger.Error(err, "failed to add namespace to ctx")
	}

	if err := ctx.AddImageInfos(&resource); err != nil {
		logger.Error(err, "unable to add image info to variables context")
	}

	if err := ctx.AddOperation("CREATE"); err != nil {
		logger.Error(err, "unable to set operation in context")
	}

	engineResponseMutation, err = mutation(policy, resource, logger, ctx, rclient, informerCacheResolvers, namespaceLabels)
	if err != nil {
		logger.Error(err, "failed to process mutation rule")
	}

	policyCtx := engine.NewPolicyContextWithJsonContext(ctx).
		WithPolicy(policy).
		WithNewResource(resource).
		WithNamespaceLabels(namespaceLabels).
		WithClient(client).
		WithExcludeGroupRole(excludeGroupRole...).
		WithInformerCacheResolver(informerCacheResolvers)

	engineResponseValidation = engine.Validate(context.TODO(), rclient, policyCtx)
	engineResponses = append(engineResponses, mergeRuleRespose(engineResponseMutation, engineResponseValidation))

	return engineResponses
}

func mutation(
	policy kyvernov1.PolicyInterface,
	resource unstructured.Unstructured,
	log logr.Logger,
	jsonContext enginecontext.Interface,
	rclient registryclient.Client,
	informerCacheResolvers resolvers.ConfigmapResolver,
	namespaceLabels map[string]string,
) (*response.EngineResponse, error) {
	policyContext := engine.NewPolicyContextWithJsonContext(jsonContext).
		WithPolicy(policy).
		WithNamespaceLabels(namespaceLabels).
		WithNewResource(resource).
		WithInformerCacheResolver(informerCacheResolvers)

	engineResponse := engine.Mutate(context.TODO(), rclient, policyContext)
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

		patch, err := jsonpatch.DecodePatch(jsonutils.JoinPatches(patches...))
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

func extractPatchPath(patches [][]byte, log logr.Logger) string {
	var resultPath []string
	// extract the patch path and value
	for _, patch := range patches {
		if data, err := jsonutils.UnmarshalPatchOperation(patch); err != nil {
			log.Error(err, "failed to decode the generate patch", "patch", string(patch))
			continue
		} else {
			resultPath = append(resultPath, data.Path)
		}
	}
	return strings.Join(resultPath, ";")
}

func mergeRuleRespose(mutation, validation *response.EngineResponse) *response.EngineResponse {
	mutation.PolicyResponse.Rules = append(mutation.PolicyResponse.Rules, validation.PolicyResponse.Rules...)
	return mutation
}
