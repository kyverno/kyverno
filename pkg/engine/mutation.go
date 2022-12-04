package engine

import (
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
	"github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(rclient registryclient.Client, policyContext *PolicyContext) (resp *response.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.policy
	resp = &response.EngineResponse{
		Policy: policy,
	}
	matchedResource := policyContext.newResource
	ctx := policyContext.jsonContext
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

		if err = MatchesResourceDescription(matchedResource, rule, policyContext.admissionInfo, excludeResource, policyContext.namespaceLabels, policyContext.policy.GetNamespace()); err != nil {
			logger.V(4).Info("rule not matched", "reason", err.Error())
			skippedRules = append(skippedRules, rule.Name)
			continue
		}

		logger.V(3).Info("processing mutate rule", "applyRules", applyRules)
		resource, err := policyContext.jsonContext.Query("request.object")
		policyContext.jsonContext.Reset()
		if err == nil && resource != nil {
			if err := ctx.AddResource(resource.(map[string]interface{})); err != nil {
				logger.Error(err, "unable to update resource object")
			}
		} else {
			logger.Error(err, "failed to query resource object")
		}

		if err := LoadContext(logger, rclient, rule.Context, policyContext, rule.Name); err != nil {
			if _, ok := err.(gojmespath.NotFoundError); ok {
				logger.V(3).Info("failed to load context", "reason", err.Error())
			} else {
				logger.Error(err, "failed to load context")
			}
			continue
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
			var mutateResp *mutate.Response
			if rule.Mutation.ForEachMutation != nil {
				m := &forEachMutator{
					rule:     ruleCopy,
					foreach:  rule.Mutation.ForEachMutation,
					ctx:      policyContext,
					resource: patchedResource,
					log:      logger,
					rclient:  rclient,
					nesting:  0,
				}

				mutateResp = m.mutateForEach()
			} else {
				mutateResp = mutateResource(ruleCopy, policyContext, patchedResource, logger)
			}

			matchedResource = mutateResp.PatchedResource
			ruleResponse := buildRuleResponse(ruleCopy, mutateResp, &patchedResource)

			if ruleResponse != nil {
				resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResponse)
				if ruleResponse.Status == response.RuleStatusError {
					incrementErrorCount(resp)
				} else {
					incrementAppliedCount(resp)
				}
			}
		}

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

func mutateResource(rule *kyvernov1.Rule, ctx *PolicyContext, resource unstructured.Unstructured, logger logr.Logger) *mutate.Response {
	preconditionsPassed, err := checkPreconditions(logger, ctx, rule.GetAnyAllConditions())
	if err != nil {
		return mutate.NewErrorResponse("failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		return mutate.NewResponse(response.RuleStatusSkip, unstructured.Unstructured{}, nil, "preconditions not met")
	}

	return mutate.Mutate(rule, ctx.JSONContext(), resource, logger)
}

type forEachMutator struct {
	rule     *kyvernov1.Rule
	ctx      *PolicyContext
	foreach  []kyvernov1.ForEachMutation
	resource unstructured.Unstructured
	nesting  int
	rclient  registryclient.Client
	log      logr.Logger
}

func (f *forEachMutator) mutateForEach() *mutate.Response {
	var applyCount int
	allPatches := make([][]byte, 0)

	for _, foreach := range f.foreach {
		if err := LoadContext(f.log, f.rclient, f.rule.Context, f.ctx, f.rule.Name); err != nil {
			f.log.Error(err, "failed to load context")
			return mutate.NewErrorResponse("failed to load context", err)
		}

		preconditionsPassed, err := checkPreconditions(f.log, f.ctx, f.rule.GetAnyAllConditions())
		if err != nil {
			return mutate.NewErrorResponse("failed to evaluate preconditions", err)

		}

		if !preconditionsPassed {
			return mutate.NewResponse(response.RuleStatusSkip, unstructured.Unstructured{}, nil, "preconditions not met")
		}

		elements, err := evaluateList(foreach.List, f.ctx.JSONContext())
		if err != nil {
			msg := fmt.Sprintf("failed to evaluate list %s: %v", foreach.List, err)
			return mutate.NewErrorResponse(msg, err)
		}

		mutateResp := f.mutateElements(foreach, elements)
		if mutateResp.Status == response.RuleStatusError {
			return mutate.NewErrorResponse("failed to mutate elements", err)
		}

		if mutateResp.Status != response.RuleStatusSkip {
			applyCount++
			if len(mutateResp.Patches) > 0 {
				f.resource = mutateResp.PatchedResource
				allPatches = append(allPatches, mutateResp.Patches...)
			}
		}
	}

	msg := fmt.Sprintf("%d elements processed", applyCount)
	if applyCount == 0 {
		return mutate.NewResponse(response.RuleStatusSkip, f.resource, allPatches, msg)
	}

	return mutate.NewResponse(response.RuleStatusPass, f.resource, allPatches, msg)
}

func (f *forEachMutator) mutateElements(foreach kyvernov1.ForEachMutation, elements []interface{}) *mutate.Response {
	f.ctx.JSONContext().Checkpoint()
	defer f.ctx.JSONContext().Restore()

	patchedResource := f.resource
	var allPatches [][]byte
	if foreach.RawPatchStrategicMerge != nil {
		invertedElement(elements)
	}

	for i, e := range elements {
		if e == nil {
			continue
		}

		f.ctx.JSONContext().Reset()
		ctx := f.ctx.Copy()

		// TODO - this needs to be refactored. The engine should not have a dependency to the CLI code
		store.SetForeachElement(i)

		falseVar := false
		if err := addElementToContext(ctx, e, i, f.nesting, &falseVar); err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to add element to mutate.foreach[%d].context", i), err)
		}

		if err := LoadContext(f.log, f.rclient, foreach.Context, ctx, f.rule.Name); err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to load to mutate.foreach[%d].context", i), err)
		}

		preconditionsPassed, err := checkPreconditions(f.log, ctx, foreach.AnyAllConditions)
		if err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to evaluate mutate.foreach[%d].preconditions", i), err)
		}

		if !preconditionsPassed {
			f.log.Info("mutate.foreach.preconditions not met", "elementIndex", i)
			continue
		}

		var mutateResp *mutate.Response
		if foreach.ForEachMutation != nil {
			nestedForeach, err := api.DeserializeJSONArray[kyvernov1.ForEachMutation](foreach.ForEachMutation)
			if err != nil {
				return mutate.NewErrorResponse("failed to deserialize foreach", err)
			}

			m := &forEachMutator{
				rule:     f.rule,
				ctx:      f.ctx,
				resource: patchedResource,
				log:      f.log,
				foreach:  nestedForeach,
				nesting:  f.nesting + 1,
			}

			mutateResp = m.mutateForEach()
		} else {
			mutateResp = mutate.ForEach(f.rule.Name, foreach, ctx.JSONContext(), patchedResource, f.log)
		}

		if mutateResp.Status == response.RuleStatusFail || mutateResp.Status == response.RuleStatusError {
			return mutateResp
		}

		if len(mutateResp.Patches) > 0 {
			patchedResource = mutateResp.PatchedResource
			allPatches = append(allPatches, mutateResp.Patches...)
		}
	}

	return mutate.NewResponse(response.RuleStatusPass, patchedResource, allPatches, "")
}

func buildRuleResponse(rule *kyvernov1.Rule, mutateResp *mutate.Response, patchedResource *unstructured.Unstructured) *response.RuleResponse {
	resp := ruleResponse(*rule, response.Mutation, mutateResp.Message, mutateResp.Status, patchedResource)
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
