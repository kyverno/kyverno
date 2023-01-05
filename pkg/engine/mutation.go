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
	"github.com/kyverno/kyverno/pkg/utils/api"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	computeRules := autogen.ComputeRules(policy)

	for i, rule := range computeRules {
		if !rule.HasMutate() {
			continue
		}
		tracing.ChildSpan(
			ctx,
			"pkg/engine",
			fmt.Sprintf("RULE %s", rule.Name),
			func(ctx context.Context, span trace.Span) {
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
					return
				}

				// check if there is a corresponding policy exception
				ruleResp := hasPolicyExceptions(policyContext, &computeRules[i], subresourceGVKToAPIResource, logger)
				if ruleResp != nil {
					resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
					return
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
					return
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
						unstructured:      matchedResource,
						subresource:       policyContext.subresource,
						parentResourceGVR: parentResourceGVR,
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
					}

					logger.V(4).Info("apply rule to resource", "rule", rule.Name, "resource namespace", patchedResource.unstructured.GetNamespace(), "resource name", patchedResource.unstructured.GetName())
					var mutateResp *mutate.Response
					if rule.Mutation.ForEachMutation != nil {
						m := &forEachMutator{
							rule:          ruleCopy,
							foreach:       rule.Mutation.ForEachMutation,
							policyContext: policyContext,
							resource:      patchedResource,
							log:           logger,
							rclient:       rclient,
							nesting:       0,
						}

						mutateResp = m.mutateForEach(ctx)
					} else {
						mutateResp = mutateResource(ruleCopy, policyContext, patchedResource.unstructured, logger)
					}

					matchedResource = mutateResp.PatchedResource
					ruleResponse := buildRuleResponse(ruleCopy, mutateResp, patchedResource)

					if ruleResponse != nil {
						resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResponse)
						if ruleResponse.Status == response.RuleStatusError {
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

func mutateResource(rule *kyvernov1.Rule, ctx *PolicyContext, resource unstructured.Unstructured, logger logr.Logger) *mutate.Response {
	preconditionsPassed, err := checkPreconditions(logger, ctx, rule.GetAnyAllConditions())
	if err != nil {
		return mutate.NewErrorResponse("failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		return mutate.NewResponse(response.RuleStatusSkip, resource, nil, "preconditions not met")
	}

	return mutate.Mutate(rule, ctx.JSONContext(), resource, logger)
}

type forEachMutator struct {
	rule          *kyvernov1.Rule
	policyContext *PolicyContext
	foreach       []kyvernov1.ForEachMutation
	resource      resourceInfo
	nesting       int
	rclient       registryclient.Client
	log           logr.Logger
}

func (f *forEachMutator) mutateForEach(ctx context.Context) *mutate.Response {
	var applyCount int
	allPatches := make([][]byte, 0)

	for _, foreach := range f.foreach {
		if err := LoadContext(ctx, f.log, f.rclient, f.rule.Context, f.policyContext, f.rule.Name); err != nil {
			f.log.Error(err, "failed to load context")
			return mutate.NewErrorResponse("failed to load context", err)
		}

		preconditionsPassed, err := checkPreconditions(f.log, f.policyContext, f.rule.GetAnyAllConditions())
		if err != nil {
			return mutate.NewErrorResponse("failed to evaluate preconditions", err)
		}

		if !preconditionsPassed {
			return mutate.NewResponse(response.RuleStatusSkip, f.resource.unstructured, nil, "preconditions not met")
		}

		elements, err := evaluateList(foreach.List, f.policyContext.JSONContext())
		if err != nil {
			msg := fmt.Sprintf("failed to evaluate list %s: %v", foreach.List, err)
			return mutate.NewErrorResponse(msg, err)
		}

		mutateResp := f.mutateElements(ctx, foreach, elements)
		if mutateResp.Status == response.RuleStatusError {
			return mutate.NewErrorResponse("failed to mutate elements", err)
		}

		if mutateResp.Status != response.RuleStatusSkip {
			applyCount++
			if len(mutateResp.Patches) > 0 {
				f.resource.unstructured = mutateResp.PatchedResource
				allPatches = append(allPatches, mutateResp.Patches...)
			}
		}
	}

	msg := fmt.Sprintf("%d elements processed", applyCount)
	if applyCount == 0 {
		return mutate.NewResponse(response.RuleStatusSkip, f.resource.unstructured, allPatches, msg)
	}

	return mutate.NewResponse(response.RuleStatusPass, f.resource.unstructured, allPatches, msg)
}

func (f *forEachMutator) mutateElements(ctx context.Context, foreach kyvernov1.ForEachMutation, elements []interface{}) *mutate.Response {
	f.policyContext.JSONContext().Checkpoint()
	defer f.policyContext.JSONContext().Restore()

	patchedResource := f.resource
	var allPatches [][]byte
	if foreach.RawPatchStrategicMerge != nil {
		invertedElement(elements)
	}

	for i, e := range elements {
		if e == nil {
			continue
		}

		f.policyContext.JSONContext().Reset()
		policyContext := f.policyContext.Copy()

		// TODO - this needs to be refactored. The engine should not have a dependency to the CLI code
		store.SetForEachElement(i)

		falseVar := false
		if err := addElementToContext(policyContext, e, i, f.nesting, &falseVar); err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to add element to mutate.foreach[%d].context", i), err)
		}

		if err := LoadContext(ctx, f.log, f.rclient, foreach.Context, policyContext, f.rule.Name); err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to load to mutate.foreach[%d].context", i), err)
		}

		preconditionsPassed, err := checkPreconditions(f.log, policyContext, foreach.AnyAllConditions)
		if err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to evaluate mutate.foreach[%d].preconditions", i), err)
		}

		if !preconditionsPassed {
			f.log.Info("mutate.foreach.preconditions not met", "elementIndex", i)
			continue
		}

		var mutateResp *mutate.Response
		if foreach.ForEachMutation != nil {
			nestedForEach, err := api.DeserializeJSONArray[kyvernov1.ForEachMutation](foreach.ForEachMutation)
			if err != nil {
				return mutate.NewErrorResponse("failed to deserialize foreach", err)
			}

			m := &forEachMutator{
				rule:          f.rule,
				policyContext: f.policyContext,
				resource:      patchedResource,
				log:           f.log,
				foreach:       nestedForEach,
				nesting:       f.nesting + 1,
			}

			mutateResp = m.mutateForEach(ctx)
		} else {
			mutateResp = mutate.ForEach(f.rule.Name, foreach, policyContext.JSONContext(), patchedResource.unstructured, f.log)
		}

		if mutateResp.Status == response.RuleStatusFail || mutateResp.Status == response.RuleStatusError {
			return mutateResp
		}

		if len(mutateResp.Patches) > 0 {
			patchedResource.unstructured = mutateResp.PatchedResource
			allPatches = append(allPatches, mutateResp.Patches...)
		}
	}

	return mutate.NewResponse(response.RuleStatusPass, patchedResource.unstructured, allPatches, "")
}

func buildRuleResponse(rule *kyvernov1.Rule, mutateResp *mutate.Response, info resourceInfo) *response.RuleResponse {
	resp := ruleResponse(*rule, response.Mutation, mutateResp.Message, mutateResp.Status)
	if resp.Status == response.RuleStatusPass {
		resp.Patches = mutateResp.Patches
		resp.Message = buildSuccessMessage(mutateResp.PatchedResource)
	}

	if len(rule.Mutation.Targets) != 0 {
		resp.PatchedTarget = &mutateResp.PatchedResource
		resp.PatchedTargetSubresourceName = info.subresource
		resp.PatchedTargetParentResourceGVR = info.parentResourceGVR
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
