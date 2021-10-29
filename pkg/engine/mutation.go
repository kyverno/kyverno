package engine

import (
	"fmt"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/pkg/errors"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// PodControllerCronJob represent CronJob string
	PodControllerCronJob = "CronJob"
	//PodControllers stores the list of Pod-controllers in csv string
	PodControllers = "DaemonSet,Deployment,Job,StatefulSet,CronJob"
	//PodControllersAnnotation defines the annotation key for Pod-Controllers
	PodControllersAnnotation = "pod-policies.kyverno.io/autogen-controllers"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	startTime := time.Now()
	policy := policyContext.Policy
	patchedResource := policyContext.NewResource
	ctx := policyContext.JSONContext

	resCache := policyContext.ResourceCache
	logger := log.Log.WithName("EngineMutate").WithValues("policy", policy.Name, "kind", patchedResource.GetKind(),
		"namespace", patchedResource.GetNamespace(), "name", patchedResource.GetName())

	logger.V(4).Info("start policy processing", "startTime", startTime)

	startMutateResultResponse(resp, policy, patchedResource)
	defer endMutateResultResponse(logger, resp, startTime)

	if ManagedPodResource(policy, patchedResource) {
		logger.V(5).Info("changes to pods managed by workload controllers are not permitted", "policy", policy.GetName())
		resp.PatchedResource = patchedResource
		return
	}

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	var err error

	for i, rule := range policy.Spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		logger := logger.WithValues("rule", rule.Name)
		excludeResource := []string{}
		if len(policyContext.ExcludeGroupRole) > 0 {
			excludeResource = policyContext.ExcludeGroupRole
		}

		if err = MatchesResourceDescription(patchedResource, rule, policyContext.AdmissionInfo, excludeResource, policyContext.NamespaceLabels, policyContext.Policy.Namespace); err != nil {
			logger.V(4).Info("rule not matched", "reason", err.Error())
			continue
		}

		logger.V(3).Info("matched mutate rule")

		// Restore() is meant for restoring context loaded from external lookup (APIServer & ConfigMap)
		// while we need to keep updated resource in the JSON context as rules can be chained
		resource, err := policyContext.JSONContext.Query("request.object")
		policyContext.JSONContext.Reset()
		if err == nil && resource != nil {
			if err := ctx.AddResourceAsObject(resource.(map[string]interface{})); err != nil {
				logger.Error(err, "unable to update resource object")
			}
		} else {
			logger.Error(err, "failed to query resource object")
		}

		if err := LoadContext(logger, rule.Context, resCache, policyContext, rule.Name); err != nil {
			if _, ok := err.(gojmespath.NotFoundError); ok {
				logger.V(3).Info("failed to load context", "reason", err.Error())
			} else {
				logger.Error(err, "failed to load context")
			}
			continue
		}

		ruleCopy := rule.DeepCopy()
		var ruleResp *response.RuleResponse
		if rule.Mutation.ForEachMutation != nil {
			ruleResp, patchedResource = mutateForEachResource(ruleCopy, policyContext, patchedResource, logger)
		} else {
			mutateResp, err := mutateResource(ruleCopy, policyContext.JSONContext, patchedResource, logger, 0)
			if err != nil {
				if mutateResp.skip {
					ruleResp = ruleResponse(&policy.Spec.Rules[i], utils.Mutation, err.Error(), response.RuleStatusSkip)
				} else {
					ruleResp = ruleResponse(&policy.Spec.Rules[i], utils.Mutation, err.Error(), response.RuleStatusError)
				}
			} else {
				if mutateResp.message == "" {
					mutateResp.message = "mutated resource"
				}

				ruleResp = ruleResponse(&policy.Spec.Rules[i], utils.Mutation, mutateResp.message, response.RuleStatusPass)
				ruleResp.Patches = mutateResp.patches
				patchedResource = mutateResp.patchedResource
			}
		}

		if ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
			if ruleResp.Status == response.RuleStatusError {
				incrementErrorCount(resp)
			} else {
				incrementAppliedCount(resp)
			}
		}
	}

	resp.PatchedResource = patchedResource
	return resp
}

func mutateForEachResource(rule *kyverno.Rule, ctx *PolicyContext, resource unstructured.Unstructured, logger logr.Logger) (*response.RuleResponse, unstructured.Unstructured) {
	foreachList := rule.Mutation.ForEachMutation
	if foreachList == nil {
		return nil, resource
	}

	applyCount := 0
	patchedResource := resource
	allPatches := make([][]byte, 0)

	for foreachIndex, foreach := range foreachList {

		if err := LoadContext(logger, foreach.Context, ctx.ResourceCache, ctx, rule.Name); err != nil {
			logger.Error(err, "failed to load context")
			return ruleError(rule, utils.Mutation, "failed to load context", err), resource
		}

		preconditionsPassed, err := checkPreconditions(logger, ctx, foreach.AnyAllConditions)
		if err != nil {
			return ruleError(rule, utils.Mutation, "failed to evaluate preconditions", err), resource
		} else if !preconditionsPassed {
			return ruleResponse(rule, utils.Mutation, "preconditions not met", response.RuleStatusSkip), resource
		}

		elements, err := evaluateList(foreach.List, ctx.JSONContext)
		if err != nil {
			msg := fmt.Sprintf("failed to evaluate list %s", foreach.List)
			return ruleError(rule, utils.Mutation, msg, err), resource
		}

		ctx.JSONContext.Checkpoint()
		defer ctx.JSONContext.Restore()

		for _, e := range elements {
			ctx.JSONContext.Reset()

			ctx := ctx.Copy()
			if err := addElementToContext(ctx, e); err != nil {
				logger.Error(err, "failed to add element to context")
				return ruleError(rule, utils.Mutation, "failed to process foreach", err), resource
			}

			var skip = false
			mutateResp, err := mutateResource(rule, ctx.JSONContext, patchedResource, logger, foreachIndex)
			if err != nil && !skip {
				return ruleResponse(rule, utils.Mutation, err.Error(), response.RuleStatusError), resource
			}

			patchedResource = mutateResp.patchedResource
			if len(mutateResp.patches) > 0 {
				allPatches = append(allPatches, mutateResp.patches...)
			}

			applyCount++
		}
	}

	if applyCount == 0 {
		return ruleResponse(rule, utils.Mutation, "0 elements processed", response.RuleStatusSkip), resource
	}

	r := ruleResponse(rule, utils.Mutation, fmt.Sprintf("%d elements processed", applyCount), response.RuleStatusPass)
	r.Patches = allPatches
	return r, patchedResource
}

type mutateResponse struct {
	skip            bool
	patchedResource unstructured.Unstructured
	patches         [][]byte
	message         string
}

func mutateResource(rule *kyverno.Rule, ctx *context.Context, resource unstructured.Unstructured, logger logr.Logger, foreachIndex int) (*mutateResponse, error) {
	mutateResp := &mutateResponse{false, unstructured.Unstructured{}, nil, ""}

	// Pre-conditions checks for the list of foreach rules should ideally be performed once.
	// Currently, they are performed for each entry in the foreach list.
	// Also, the foreach index parameter should be removed and a set of patches should be passed in.
	anyAllConditions, err := variables.SubstituteAllInPreconditions(logger, ctx, rule.AnyAllConditions)
	if err != nil {
		return mutateResp, errors.Wrapf(err, "failed to substitute vars in preconditions")
	}

	copyConditions, err := transformConditions(anyAllConditions)
	if err != nil {
		return mutateResp, errors.Wrapf(err, "failed to load context")
	}

	if !variables.EvaluateConditions(logger, ctx, copyConditions) {
		mutateResp.skip = true
		return mutateResp, fmt.Errorf("preconditions mismatch")
	}

	updatedRule, err := variables.SubstituteAllInRule(logger, ctx, *rule)
	if err != nil {
		return mutateResp, errors.Wrapf(err, "variable substitution failed")
	}

	mutation := updatedRule.Mutation.DeepCopy()
	mutateHandler := mutate.CreateMutateHandler(updatedRule.Name, mutation, resource, ctx, logger, foreachIndex)
	resp, patchedResource := mutateHandler.Handle()

	if resp.Status == response.RuleStatusPass {
		// - overlay pattern does not match the resource conditions
		if resp.Patches == nil {
			mutateResp.skip = true
			return mutateResp, fmt.Errorf("resource does not match pattern")
		}

		mutateResp.skip = false
		mutateResp.patchedResource = patchedResource
		mutateResp.patches = resp.Patches
		mutateResp.message = resp.Message
		logger.V(4).Info("mutate rule applied successfully", "ruleName", rule.Name)
	}

	if err := ctx.AddResourceAsObject(patchedResource.Object); err != nil {
		logger.Error(err, "failed to update resource in the JSON context")
	}

	return mutateResp, nil
}

func startMutateResultResponse(resp *response.EngineResponse, policy kyverno.ClusterPolicy, resource unstructured.Unstructured) {
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
