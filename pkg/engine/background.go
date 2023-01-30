package engine

import (
	"context"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

// ApplyBackgroundChecks checks for validity of generate and mutateExisting rules on the resource
// 1. validate variables to be substitute in the general ruleInfo (match,exclude,condition)
//   - the caller has to check the ruleResponse to determine whether the path exist
//
// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
func ApplyBackgroundChecks(rclient registryclient.Client, policyContext *PolicyContext) (resp *engineapi.EngineResponse) {
	policyStartTime := time.Now()
	return filterRules(rclient, policyContext, policyStartTime)
}

func filterRules(rclient registryclient.Client, policyContext *PolicyContext, startTime time.Time) *engineapi.EngineResponse {
	kind := policyContext.newResource.GetKind()
	name := policyContext.newResource.GetName()
	namespace := policyContext.newResource.GetNamespace()
	apiVersion := policyContext.newResource.GetAPIVersion()
	resp := &engineapi.EngineResponse{
		PolicyResponse: engineapi.PolicyResponse{
			Policy: engineapi.PolicySpec{
				Name:      policyContext.policy.GetName(),
				Namespace: policyContext.policy.GetNamespace(),
			},
			PolicyStats: engineapi.PolicyStats{
				PolicyExecutionTimestamp: startTime.Unix(),
			},
			Resource: engineapi.ResourceSpec{
				Kind:       kind,
				Name:       name,
				Namespace:  namespace,
				APIVersion: apiVersion,
			},
		},
	}

	if policyContext.excludeResourceFunc(kind, namespace, name) {
		logging.WithName("ApplyBackgroundChecks").Info("resource excluded", "kind", kind, "namespace", namespace, "name", name)
		return resp
	}

	applyRules := policyContext.policy.GetSpec().GetApplyRules()
	for _, rule := range autogen.ComputeRules(policyContext.policy) {
		if ruleResp := filterRule(rclient, rule, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
			if applyRules == kyvernov1.ApplyOne && ruleResp.Status != engineapi.RuleStatusSkip {
				break
			}
		}
	}

	return resp
}

func filterRule(rclient registryclient.Client, rule kyvernov1.Rule, policyContext *PolicyContext) *engineapi.RuleResponse {
	if !rule.HasGenerate() && !rule.IsMutateExisting() {
		return nil
	}

	logger := logging.WithName("exception")

	kindsInPolicy := append(rule.MatchResources.GetKinds(), rule.ExcludeResources.GetKinds()...)
	subresourceGVKToAPIResource := GetSubresourceGVKToAPIResourceMap(kindsInPolicy, policyContext)

	// check if there is a corresponding policy exception
	ruleResp := hasPolicyExceptions(policyContext, &rule, subresourceGVKToAPIResource, logger)
	if ruleResp != nil {
		return ruleResp
	}

	ruleType := engineapi.Mutation
	if rule.HasGenerate() {
		ruleType = engineapi.Generation
	}

	startTime := time.Now()

	policy := policyContext.policy
	newResource := policyContext.newResource
	oldResource := policyContext.oldResource
	admissionInfo := policyContext.admissionInfo
	ctx := policyContext.jsonContext
	excludeGroupRole := policyContext.excludeGroupRole
	namespaceLabels := policyContext.namespaceLabels

	logger = logging.WithName(string(ruleType)).WithValues("policy", policy.GetName(),
		"kind", newResource.GetKind(), "namespace", newResource.GetNamespace(), "name", newResource.GetName())

	if err := MatchesResourceDescription(subresourceGVKToAPIResource, newResource, rule, admissionInfo, excludeGroupRole, namespaceLabels, "", policyContext.subresource); err != nil {
		if ruleType == engineapi.Generation {
			// if the oldResource matched, return "false" to delete GR for it
			if err = MatchesResourceDescription(subresourceGVKToAPIResource, oldResource, rule, admissionInfo, excludeGroupRole, namespaceLabels, "", policyContext.subresource); err == nil {
				return &engineapi.RuleResponse{
					Name:   rule.Name,
					Type:   ruleType,
					Status: engineapi.RuleStatusFail,
					RuleStats: engineapi.RuleStats{
						ProcessingTime:         time.Since(startTime),
						RuleExecutionTimestamp: startTime.Unix(),
					},
				}
			}
		}
		logger.V(4).Info("rule not matched", "reason", err.Error())
		return nil
	}

	policyContext.jsonContext.Checkpoint()
	defer policyContext.jsonContext.Restore()

	if err := LoadContext(context.TODO(), logger, rclient, rule.Context, policyContext, rule.Name); err != nil {
		logger.V(4).Info("cannot add external data to the context", "reason", err.Error())
		return nil
	}

	ruleCopy := rule.DeepCopy()
	if after, err := variables.SubstituteAllInPreconditions(logger, ctx, ruleCopy.GetAnyAllConditions()); err != nil {
		logger.V(4).Info("failed to substitute vars in preconditions, skip current rule", "rule name", ruleCopy.Name)
		return nil
	} else {
		ruleCopy.SetAnyAllConditions(after)
	}

	// operate on the copy of the conditions, as we perform variable substitution
	copyConditions, err := common.TransformConditions(ruleCopy.GetAnyAllConditions())
	if err != nil {
		logger.V(4).Info("cannot copy AnyAllConditions", "reason", err.Error())
		return nil
	}

	// evaluate pre-conditions
	if !variables.EvaluateConditions(logger, ctx, copyConditions) {
		logger.V(4).Info("skip rule as preconditions are not met", "rule", ruleCopy.Name)
		return &engineapi.RuleResponse{
			Name:   ruleCopy.Name,
			Type:   ruleType,
			Status: engineapi.RuleStatusSkip,
			RuleStats: engineapi.RuleStats{
				ProcessingTime:         time.Since(startTime),
				RuleExecutionTimestamp: startTime.Unix(),
			},
		}
	}

	// build rule Response
	return &engineapi.RuleResponse{
		Name:   ruleCopy.Name,
		Type:   ruleType,
		Status: engineapi.RuleStatusPass,
		RuleStats: engineapi.RuleStats{
			ProcessingTime:         time.Since(startTime),
			RuleExecutionTimestamp: startTime.Unix(),
		},
	}
}
