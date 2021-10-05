package engine

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
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

	for _, rule := range policy.Spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		var ruleResponse response.RuleResponse
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
				logger.WithName("RestoreContext").Error(err, "unable to update resource object")
			}
		} else {
			logger.WithName("RestoreContext").Error(err, "failed to query resource object")
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
		ruleCopy.AnyAllConditions, err = variables.SubstituteAllInPreconditions(logger, ctx, ruleCopy.AnyAllConditions)
		if err != nil {
			logger.V(3).Info("failed to substitute vars in preconditions, skip current rule", "rule name", rule.Name)
			continue
		}

		// operate on the copy of the conditions, as we perform variable substitution
		copyConditions, err := transformConditions(ruleCopy.AnyAllConditions)
		if err != nil {
			logger.V(2).Info("failed to load context", "reason", err.Error())
			continue
		}
		// evaluate pre-conditions
		// - handle variable substitutions
		if !variables.EvaluateConditions(logger, ctx, copyConditions) {
			logger.V(3).Info("resource fails the preconditions")
			continue
		}

		if *ruleCopy, err = variables.SubstituteAllInRule(logger, ctx, *ruleCopy); err != nil {
			ruleResp := response.RuleResponse{
				Name:    ruleCopy.Name,
				Type:    utils.Mutation.String(),
				Message: fmt.Sprintf("variable substitution failed: %s", err.Error()),
				Status:  response.RuleStatusPass,
			}

			incrementAppliedCount(resp)
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResp)

			logger.Error(err, "failed to substitute variables, skip current rule", "rule name", ruleCopy.Name)
			continue
		}

		mutation := ruleCopy.Mutation.DeepCopy()
		mutateHandler := mutate.CreateMutateHandler(ruleCopy.Name, mutation, patchedResource, ctx, logger)
		ruleResponse, patchedResource = mutateHandler.Handle()
		if ruleResponse.Status == response.RuleStatusPass {
			// - overlay pattern does not match the resource conditions
			if ruleResponse.Patches == nil {
				continue
			}

			logger.V(4).Info("mutate rule applied successfully", "ruleName", ruleCopy.Name)
		}

		if err := ctx.AddResourceAsObject(patchedResource.Object); err != nil {
			logger.Error(err, "failed to update resource in the JSON context")
		}

		resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
		incrementAppliedRuleCount(resp)
	}

	resp.PatchedResource = patchedResource
	return resp
}

func incrementAppliedRuleCount(resp *response.EngineResponse) {
	resp.PolicyResponse.RulesAppliedCount++
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
