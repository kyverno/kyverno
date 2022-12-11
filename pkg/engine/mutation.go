package engine

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(ctx context.Context, rclient registryclient.Client, policyContext *PolicyContext) (resp *response.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.policy
	resp = &response.EngineResponse{
		Policy: policy,
	}
	matchedResource := policyContext.newResource
	enginectx := policyContext.jsonContext
	var skippedRules []string

	logger := logging.WithName("EngineMutate").WithValues("policy", policy.GetName(), "kind", matchedResource.GetKind(),
		"namespace", matchedResource.GetNamespace(), "name", matchedResource.GetName())

	logger.V(4).Info("start mutate policy processing", "startTime", startTime)

	startMutateResultResponse(resp, policy, matchedResource)
	defer endMutateResultResponse(logger, resp, startTime)

	policyContext.jsonContext.Checkpoint()
	defer policyContext.jsonContext.Restore()

	var err error
	applyRules := policy.GetSpec().GetApplyRules()

	for _, rule := range autogen.ComputeRules(policy) {
		if !rule.HasMutate() {
			continue
		}

		logger := logger.WithValues("rule", rule.Name)
		var excludeResource []string
		if len(policyContext.excludeGroupRole) > 0 {
			excludeResource = policyContext.excludeGroupRole
		}

		kindsInPolicy := append(rule.MatchResources.GetKinds(), rule.ExcludeResources.GetKinds()...)
		subresourceGVKToAPIResource := GetSubresourceGVKToAPIResourceMap(kindsInPolicy, policyContext)
		if err = MatchesResourceDescription(subresourceGVKToAPIResource, matchedResource, rule, policyContext.admissionInfo, excludeResource, policyContext.namespaceLabels, policyContext.policy.GetNamespace(), policyContext.subresource); err != nil {
			logger.V(4).Info("rule not matched", "reason", err.Error())
			skippedRules = append(skippedRules, rule.Name)
			continue
		}

		logger.V(3).Info("processing mutate rule", "applyRules", applyRules)
		resource, err := policyContext.jsonContext.Query("request.object")
		policyContext.jsonContext.Reset()
		if err == nil && resource != nil {
			if err := enginectx.AddResource(resource.(map[string]interface{})); err != nil {
				logger.Error(err, "unable to update resource object")
			}
		} else {
			logger.Error(err, "failed to query resource object")
		}

		if err := LoadContext(ctx, logger, rclient, rule.Context, policyContext, rule.Name); err != nil {
			if _, ok := err.(gojmespath.NotFoundError); ok {
				logger.V(3).Info("failed to load context", "reason", err.Error())
			} else {
				logger.Error(err, "failed to load context")
			}
			continue
		}

		ruleCopy := rule.DeepCopy()
		var patchedResources []resourceInfo
		if !policyContext.admissionOperation && rule.IsMutateExisting() {
			targets, err := loadTargets(ruleCopy.Mutation.Targets, policyContext, logger)
			if err != nil {
				rr := ruleResponse(rule, response.Mutation, err.Error(), response.RuleStatusError)
				resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *rr)
			} else {
				patchedResources = append(patchedResources, targets...)
			}
		} else {
			var parentResourceGVR metav1.GroupVersionResource
			if policyContext.subresource != "" {
				parentResourceGVR = policyContext.requestResource
			}
			patchedResources = append(patchedResources, resourceInfo{
				unstructured: matchedResource, subresource: policyContext.subresource, parentResourceGVR: parentResourceGVR,
			})
		}

		for _, patchedResource := range patchedResources {
			if reflect.DeepEqual(patchedResource, unstructured.Unstructured{}) {
				continue
			}

			if !policyContext.admissionOperation && rule.IsMutateExisting() {
				policyContext := policyContext.Copy()
				if err := policyContext.jsonContext.AddTargetResource(patchedResource.unstructured.Object); err != nil {
					logging.Error(err, "failed to add target resource to the context")
					continue
				}

			logger.V(4).Info("apply rule to resource", "rule", rule.Name, "resource namespace", patchedResource.unstructured.GetNamespace(), "resource name", patchedResource.unstructured.GetName())
			var ruleResp *response.RuleResponse
			if rule.Mutation.ForEachMutation != nil {
				ruleResp, patchedResource.unstructured = mutateForEach(ctx, rclient, ruleCopy, policyContext, patchedResource.unstructured, patchedResource.subresource, patchedResource.parentResourceGVR, logger)
			} else {
				ruleResp, patchedResource.unstructured = mutateResource(ruleCopy, policyContext, patchedResource.unstructured, patchedResource.subresource, patchedResource.parentResourceGVR, logger)
			}

			matchedResource = patchedResource.unstructured

			if ruleResp != nil {
				resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
				if ruleResp.Status == response.RuleStatusError {
					incrementErrorCount(resp)
				} else {
					logger.Error(err, "failed to query resource object")
				}

				if err := LoadContext(ctx, logger, rclient, rule.Context, policyContext, rule.Name); err != nil {
					if _, ok := err.(gojmespath.NotFoundError); ok {
						logger.V(3).Info("failed to load context", "reason", err.Error())
					} else {
						logger.Error(err, "failed to load context")
					}
					return
				}

				ruleCopy := rule.DeepCopy()
				var patchedResources []unstructured.Unstructured
				if !policyContext.admissionOperation && rule.IsMutateExisting() {
					targets, err := loadTargets(ruleCopy.Mutation.Targets, policyContext, logger)
					if err != nil {
						rr := ruleResponse(rule, response.Mutation, err.Error(), response.RuleStatusError, nil)
						resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *rr)
					} else {
						patchedResources = append(patchedResources, targets...)
					}
				} else {
					patchedResources = append(patchedResources, matchedResource)
				}

				for _, patchedResource := range patchedResources {
					if reflect.DeepEqual(patchedResource, unstructured.Unstructured{}) {
						continue
					}

					if !policyContext.admissionOperation && rule.IsMutateExisting() {
						policyContext := policyContext.Copy()
						if err := policyContext.jsonContext.AddTargetResource(patchedResource.Object); err != nil {
							logging.Error(err, "failed to add target resource to the context")
							continue
						}
					}

					logger.V(4).Info("apply rule to resource", "rule", rule.Name, "resource namespace", patchedResource.GetNamespace(), "resource name", patchedResource.GetName())
					var ruleResp *response.RuleResponse
					if rule.Mutation.ForEachMutation != nil {
						ruleResp, patchedResource = mutateForEach(ctx, rclient, ruleCopy, policyContext, patchedResource, logger)
					} else {
						ruleResp, patchedResource = mutateResource(ruleCopy, policyContext, patchedResource, logger)
					}

					matchedResource = patchedResource

					if ruleResp != nil {
						resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
						if ruleResp.Status == response.RuleStatusError {
							incrementErrorCount(resp)
						} else {
							incrementAppliedCount(resp)
						}
					}
				}
			},
		)

		if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.RulesAppliedCount > 0 {
			break
		}
	}

	for _, r := range resp.PolicyResponse.Rules {
		for _, n := range skippedRules {
			if r.Name == n {
				r.Status = response.RuleStatusSkip
				logger.V(4).Info("rule Status set as skip", "rule skippedRules", r.Name)
			}
		}
	}

	resp.PatchedResource = matchedResource
	return resp
}

func mutateResource(rule *kyvernov1.Rule, ctx *PolicyContext, resource unstructured.Unstructured, subresourceName string, parentResourceGVR metav1.GroupVersionResource, logger logr.Logger) (*response.RuleResponse, unstructured.Unstructured) {
	preconditionsPassed, err := checkPreconditions(logger, ctx, rule.GetAnyAllConditions())
	if err != nil {
		return ruleError(rule, response.Mutation, "failed to evaluate preconditions", err), resource
	}

	if !preconditionsPassed {
		return ruleResponseWithPatchedTarget(*rule, response.Mutation, "preconditions not met", response.RuleStatusSkip, &resource, subresourceName, parentResourceGVR), resource
	}

	mutateResp := mutate.Mutate(rule, ctx.jsonContext, resource, logger)
	ruleResp := buildRuleResponse(rule, mutateResp, &mutateResp.PatchedResource, subresourceName, parentResourceGVR)
	return ruleResp, mutateResp.PatchedResource
}

func mutateForEach(ctx context.Context, rclient registryclient.Client, rule *kyvernov1.Rule, enginectx *PolicyContext, resource unstructured.Unstructured, subresourceName string, parentResourceGVR metav1.GroupVersionResource, logger logr.Logger) (*response.RuleResponse, unstructured.Unstructured) {
	foreachList := rule.Mutation.ForEachMutation
	if foreachList == nil {
		return nil, resource
	}

	patchedResource := resource
	var applyCount int
	allPatches := make([][]byte, 0)

	for _, foreach := range foreachList {
		if err := LoadContext(ctx, logger, rclient, rule.Context, enginectx, rule.Name); err != nil {
			logger.Error(err, "failed to load context")
			return ruleError(rule, response.Mutation, "failed to load context", err), resource
		}

		preconditionsPassed, err := checkPreconditions(logger, enginectx, rule.GetAnyAllConditions())
		if err != nil {
			return ruleError(rule, response.Mutation, "failed to evaluate preconditions", err), resource
		}

		if !preconditionsPassed {
			return ruleResponseWithPatchedTarget(*rule, response.Mutation, "preconditions not met", response.RuleStatusSkip, &patchedResource, subresourceName, parentResourceGVR), resource
		}

		elements, err := evaluateList(foreach.List, enginectx.jsonContext)
		if err != nil {
			msg := fmt.Sprintf("failed to evaluate list %s", foreach.List)
			return ruleError(rule, response.Mutation, msg, err), resource
		}

		mutateResp := mutateElements(ctx, rclient, rule.Name, foreach, enginectx, elements, patchedResource, logger)
		if mutateResp.Status == response.RuleStatusError {
			logger.Error(err, "failed to mutate elements")
			return buildRuleResponse(rule, mutateResp, nil, "", metav1.GroupVersionResource{}), resource
		}

		if mutateResp.Status != response.RuleStatusSkip {
			applyCount++
			if len(mutateResp.Patches) > 0 {
				patchedResource = mutateResp.PatchedResource
				allPatches = append(allPatches, mutateResp.Patches...)
			}
		}
	}

	if applyCount == 0 {
		return ruleResponseWithPatchedTarget(*rule, response.Mutation, "0 elements processed", response.RuleStatusSkip, &resource, subresourceName, parentResourceGVR), resource
	}

	r := ruleResponseWithPatchedTarget(*rule, response.Mutation, fmt.Sprintf("%d elements processed", applyCount), response.RuleStatusPass, &patchedResource, subresourceName, parentResourceGVR)
	r.Patches = allPatches
	return r, patchedResource
}

func mutateElements(ctx context.Context, rclient registryclient.Client, name string, foreach kyvernov1.ForEachMutation, enginectx *PolicyContext, elements []interface{}, resource unstructured.Unstructured, logger logr.Logger) *mutate.Response {
	enginectx.jsonContext.Checkpoint()
	defer enginectx.jsonContext.Restore()

	patchedResource := resource
	var allPatches [][]byte
	if foreach.RawPatchStrategicMerge != nil {
		invertedElement(elements)
	}

	for i, e := range elements {
		if e == nil {
			continue
		}
		enginectx.jsonContext.Reset()
		enginectx := enginectx.Copy()
		store.SetForeachElement(i)
		falseVar := false
		if err := addElementToContext(enginectx, e, i, &falseVar); err != nil {
			return mutateError(err, fmt.Sprintf("failed to add element to mutate.foreach[%d].context", i))
		}

		if err := LoadContext(ctx, logger, rclient, foreach.Context, enginectx, name); err != nil {
			return mutateError(err, fmt.Sprintf("failed to load to mutate.foreach[%d].context", i))
		}

		preconditionsPassed, err := checkPreconditions(logger, enginectx, foreach.AnyAllConditions)
		if err != nil {
			return mutateError(err, fmt.Sprintf("failed to evaluate mutate.foreach[%d].preconditions", i))
		}

		if !preconditionsPassed {
			logger.Info("mutate.foreach.preconditions not met", "elementIndex", i)
			continue
		}

		mutateResp := mutate.ForEach(name, foreach, enginectx.jsonContext, patchedResource, logger)
		if mutateResp.Status == response.RuleStatusFail || mutateResp.Status == response.RuleStatusError {
			return mutateResp
		}

		if len(mutateResp.Patches) > 0 {
			patchedResource = mutateResp.PatchedResource
			allPatches = append(allPatches, mutateResp.Patches...)
		}
	}

	return &mutate.Response{
		Status:          response.RuleStatusPass,
		PatchedResource: patchedResource,
		Patches:         allPatches,
		Message:         "foreach mutation applied",
	}
}

func mutateError(err error, message string) *mutate.Response {
	return &mutate.Response{
		Status:          response.RuleStatusFail,
		PatchedResource: unstructured.Unstructured{},
		Patches:         nil,
		Message:         fmt.Sprintf("failed to add element to context: %v", err),
	}
}

func buildRuleResponse(rule *kyvernov1.Rule, mutateResp *mutate.Response, patchedResource *unstructured.Unstructured, patchedSubresourceName string, parentResourceGVR metav1.GroupVersionResource) *response.RuleResponse {
	resp := ruleResponseWithPatchedTarget(*rule, response.Mutation, mutateResp.Message, mutateResp.Status, patchedResource, patchedSubresourceName, parentResourceGVR)
	if resp.Status == response.RuleStatusPass {
		resp.Patches = mutateResp.Patches
		resp.Message = buildSuccessMessage(mutateResp.PatchedResource)
	}

	return resp
}

func buildSuccessMessage(r unstructured.Unstructured) string {
	if reflect.DeepEqual(unstructured.Unstructured{}, r) {
		return "mutated resource"
	}

	if r.GetNamespace() == "" {
		return fmt.Sprintf("mutated %s/%s", r.GetKind(), r.GetName())
	}

	return fmt.Sprintf("mutated %s/%s in namespace %s", r.GetKind(), r.GetName(), r.GetNamespace())
}

func startMutateResultResponse(resp *response.EngineResponse, policy kyvernov1.PolicyInterface, resource unstructured.Unstructured) {
	if resp == nil {
		return
	}

	resp.PolicyResponse.Policy.Name = policy.GetName()
	resp.PolicyResponse.Policy.Namespace = policy.GetNamespace()
	resp.PolicyResponse.Resource.Name = resource.GetName()
	resp.PolicyResponse.Resource.Namespace = resource.GetNamespace()
	resp.PolicyResponse.Resource.Kind = resource.GetKind()
	resp.PolicyResponse.Resource.APIVersion = resource.GetAPIVersion()
}

func endMutateResultResponse(logger logr.Logger, resp *response.EngineResponse, startTime time.Time) {
	if resp == nil {
		return
	}

	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.PolicyExecutionTimestamp = startTime.Unix()
	logger.V(5).Info("finished processing policy", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "mutationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
}
