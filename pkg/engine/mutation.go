package engine

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kyverno/kyverno/pkg/engine/mutate"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
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
	var skippedRules []string

	logger := log.Log.WithName("EngineMutate").WithValues("policy", policy.Name, "kind", patchedResource.GetKind(),
		"namespace", patchedResource.GetNamespace(), "skippedRules", patchedResource.GetName())

	logger.V(4).Info("start policy processing", "startTime", startTime)

	startMutateResultResponse(resp, policy, patchedResource)
	defer endMutateResultResponse(logger, resp, startTime)

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	var err error

	for _, rule := range policy.Spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		logger := logger.WithValues("rule", rule.Name)
		var excludeResource []string
		if len(policyContext.ExcludeGroupRole) > 0 {
			excludeResource = policyContext.ExcludeGroupRole
		}

		if err = MatchesResourceDescription(patchedResource, rule, policyContext.AdmissionInfo, excludeResource, policyContext.NamespaceLabels, policyContext.Policy.Namespace); err != nil {
			logger.V(4).Info("rule not matched", "reason", err.Error())
			skippedRules = append(skippedRules, rule.Name)
			continue
		}

		logger.V(3).Info("matched mutate rule")

		resource, err := policyContext.JSONContext.Query("request.object")
		policyContext.JSONContext.Reset()
		if err == nil && resource != nil {
			if err := ctx.AddResourceAsObject(resource.(map[string]interface{})); err != nil {
				logger.Error(err, "unable to update resource object")
			}
		} else {
			logger.Error(err, "failed to query resource object")
		}

		if err := LoadContext(logger, rule.Context, policyContext, rule.Name); err != nil {
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
			ruleResp, patchedResource = mutateForEach(ruleCopy, policyContext, patchedResource, logger)
		} else {
			ruleResp, patchedResource = mutateResource(ruleCopy, policyContext, patchedResource, logger)
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

	for _, r := range resp.PolicyResponse.Rules {
		for _, n := range skippedRules {
			if r.Name == n {
				r.Status = response.RuleStatusSkip
				logger.V(4).Info("rule Status set as skip", "rule skippedRules", r.Name)
			}
		}
	}

	resp.PatchedResource = patchedResource
	return resp
}

func mutateResource(rule *kyverno.Rule, ctx *PolicyContext, resource unstructured.Unstructured, logger logr.Logger) (*response.RuleResponse, unstructured.Unstructured) {
	preconditionsPassed, err := checkPreconditions(logger, ctx, rule.AnyAllConditions)
	if err != nil {
		return ruleError(rule, utils.Mutation, "failed to evaluate preconditions", err), resource
	}

	if !preconditionsPassed {
		return ruleResponse(rule, utils.Mutation, "preconditions not met", response.RuleStatusSkip), resource
	}

	mutateResp := mutate.Mutate(rule, ctx.JSONContext, resource, logger)
	ruleResp := buildRuleResponse(rule, mutateResp)
	return ruleResp, mutateResp.PatchedResource
}

func mutateForEach(rule *kyverno.Rule, ctx *PolicyContext, resource unstructured.Unstructured, logger logr.Logger) (*response.RuleResponse, unstructured.Unstructured) {
	foreachList := rule.Mutation.ForEachMutation
	if foreachList == nil {
		return nil, resource
	}

	patchedResource := resource
	var applyCount int
	allPatches := make([][]byte, 0)

	for _, foreach := range foreachList {
		if err := LoadContext(logger, rule.Context, ctx, rule.Name); err != nil {
			logger.Error(err, "failed to load context")
			return ruleError(rule, utils.Mutation, "failed to load context", err), resource
		}

		preconditionsPassed, err := checkPreconditions(logger, ctx, rule.AnyAllConditions)
		if err != nil {
			return ruleError(rule, utils.Mutation, "failed to evaluate preconditions", err), resource
		}

		if !preconditionsPassed {
			return ruleResponse(rule, utils.Mutation, "preconditions not met", response.RuleStatusSkip), resource
		}

		elements, err := evaluateList(foreach.List, ctx.JSONContext)
		if err != nil {
			msg := fmt.Sprintf("failed to evaluate list %s", foreach.List)
			return ruleError(rule, utils.Mutation, msg, err), resource
		}

		mutateResp := mutateElements(rule.Name, foreach, ctx, elements, patchedResource, logger)
		if mutateResp.Status == response.RuleStatusError {
			logger.Error(err, "failed to mutate elements")
			return buildRuleResponse(rule, mutateResp), resource
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
		return ruleResponse(rule, utils.Mutation, "0 elements processed", response.RuleStatusSkip), resource
	}

	r := ruleResponse(rule, utils.Mutation, fmt.Sprintf("%d elements processed", applyCount), response.RuleStatusPass)
	r.Patches = allPatches
	return r, patchedResource
}

func mutateElements(name string, foreach *kyverno.ForEachMutation, ctx *PolicyContext, elements []interface{}, resource unstructured.Unstructured, logger logr.Logger) *mutate.Response {
	ctx.JSONContext.Checkpoint()
	defer ctx.JSONContext.Restore()

	patchedResource := resource
	var allPatches [][]byte

	for i, e := range elements {
		ctx.JSONContext.Reset()
		ctx := ctx.Copy()
		if err := addElementToContext(ctx, e, i, false); err != nil {
			return mutateError(err, fmt.Sprintf("failed to add element to mutate.foreach[%d].context", i))
		}

		if err := LoadContext(logger, foreach.Context, ctx, name); err != nil {
			return mutateError(err, fmt.Sprintf("failed to load to mutate.foreach[%d].context", i))
		}

		preconditionsPassed, err := checkPreconditions(logger, ctx, foreach.AnyAllConditions)
		if err != nil {
			return mutateError(err, fmt.Sprintf("failed to evaluate mutate.foreach[%d].preconditions", i))
		}

		if !preconditionsPassed {
			logger.Info("mutate.foreach.preconditions not met", "elementIndex", i)
			continue
		}

		mutateResp := mutate.ForEach(name, foreach, ctx.JSONContext, patchedResource, logger)
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

func buildRuleResponse(rule *kyverno.Rule, mutateResp *mutate.Response) *response.RuleResponse {
	resp := ruleResponse(rule, utils.Mutation, mutateResp.Message, mutateResp.Status)
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

	ns := r.GetNamespace()
	if ns == "" {
		return fmt.Sprintf("mutated %s/%s", r.GetKind(), r.GetName())
	}

	return fmt.Sprintf("mutated %s/%s in namespace %s", r.GetKind(), r.GetName(), r.GetNamespace())
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
