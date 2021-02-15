package engine

import (
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/response"
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
		logger.V(5).Info("skip applying policy as direct changes to pods managed by workload controllers are not allowed", "policy", policy.GetName())
		resp.PatchedResource = patchedResource
		return
	}

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	for _, rule := range policy.Spec.Rules {
		var ruleResponse response.RuleResponse
		logger := logger.WithValues("rule", rule.Name)
		if !rule.HasMutate() {
			continue
		}

		// check if the resource satisfies the filter conditions defined in the rule
		//TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont satisfy a policy rule resource description
		excludeResource := []string{}
		if len(policyContext.ExcludeGroupRole) > 0 {
			excludeResource = policyContext.ExcludeGroupRole
		}

		if err := MatchesResourceDescription(patchedResource, rule, policyContext.AdmissionInfo, excludeResource, policyContext.NamespaceLabels); err != nil {
			logger.V(4).Info("rule not matched", "reason", err.Error())
			continue
		}

		logger.V(3).Info("matched mutate rule")

		policyContext.JSONContext.Restore()
		if err := LoadContext(logger, rule.Context, resCache, policyContext); err != nil {
			logger.Error(err, "failed to load context")
			continue
		}

		// operate on the copy of the conditions, as we perform variable substitution
		copyConditions, err := copyConditions(rule.AnyAllConditions)
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

		mutation := rule.Mutation.DeepCopy()
		mutateHandler := mutate.CreateMutateHandler(rule.Name, mutation, patchedResource, ctx, logger)
		ruleResponse, patchedResource = mutateHandler.Handle()
		if ruleResponse.Success {
			// - overlay pattern does not match the resource conditions
			if ruleResponse.Patches == nil {
				continue
			}

			logger.V(4).Info("mutate rule applied successfully", "ruleName", rule.Name)
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

	resp.PolicyResponse.Policy = policy.Name
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
	logger.V(5).Info("finished processing policy", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "mutationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
}
