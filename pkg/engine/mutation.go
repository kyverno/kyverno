package engine

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/kyverno/kyverno/pkg/utils/api"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func (e *engine) mutate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) (resp *engineapi.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.Policy()
	resp = engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	matchedResource := policyContext.NewResource()
	enginectx := policyContext.JSONContext()
	var skippedRules []string

	logger.V(4).Info("start mutate policy processing", "startTime", startTime)

	startMutateResultResponse(resp, policy, matchedResource)
	defer endMutateResultResponse(logger, resp, startTime)

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

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
				logger := internal.LoggerWithRule(logger, rule)
				var excludeResource []string
				if len(e.configuration.GetExcludeGroupRole()) > 0 {
					excludeResource = e.configuration.GetExcludeGroupRole()
				}

				kindsInPolicy := append(rule.MatchResources.GetKinds(), rule.ExcludeResources.GetKinds()...)
				subresourceGVKToAPIResource := GetSubresourceGVKToAPIResourceMap(e.client, kindsInPolicy, policyContext)
				if err = MatchesResourceDescription(subresourceGVKToAPIResource, matchedResource, rule, policyContext.AdmissionInfo(), excludeResource, policyContext.NamespaceLabels(), policyContext.Policy().GetNamespace(), policyContext.SubResource()); err != nil {
					logger.V(4).Info("rule not matched", "reason", err.Error())
					skippedRules = append(skippedRules, rule.Name)
					return
				}

				// check if there is a corresponding policy exception
				if ruleResp := hasPolicyExceptions(logger, engineapi.Mutation, e.exceptionSelector, policyContext, &computeRules[i], subresourceGVKToAPIResource, e.configuration); ruleResp != nil {
					resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
					return
				}

				logger.V(3).Info("processing mutate rule")
				resource, err := policyContext.JSONContext().Query("request.object")
				logger.Info("resource", "resource", resource)
				policyContext.JSONContext().Reset()
				if err == nil && resource != nil {
					if err := enginectx.AddResource(resource.(map[string]interface{})); err != nil {
						logger.Error(err, "unable to update resource object")
					}
				} else {
					logger.Error(err, "failed to query resource object")
				}

				if err := internal.LoadContext(ctx, e, policyContext, rule); err != nil {
					if _, ok := err.(gojmespath.NotFoundError); ok {
						logger.V(3).Info("failed to load context", "reason", err.Error())
					} else {
						logger.Error(err, "failed to load context")
					}
					return
				}

				ruleCopy := rule.DeepCopy()
				var patchedResources []resourceInfo
				if !policyContext.AdmissionOperation() && rule.IsMutateExisting() {
					targets, err := loadTargets(e.client, ruleCopy.Mutation.Targets, policyContext, logger)
					if err != nil {
						rr := internal.RuleError(ruleCopy, engineapi.Mutation, "", err)
						resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *rr)
					} else {
						patchedResources = append(patchedResources, targets...)
					}
				} else {
					var parentResourceGVR metav1.GroupVersionResource
					if policyContext.SubResource() != "" {
						parentResourceGVR = policyContext.RequestResource()
					}
					patchedResources = append(patchedResources, resourceInfo{
						unstructured:      matchedResource,
						subresource:       policyContext.SubResource(),
						parentResourceGVR: parentResourceGVR,
					})
				}

				for _, patchedResource := range patchedResources {
					if reflect.DeepEqual(patchedResource, unstructured.Unstructured{}) {
						continue
					}

					if !policyContext.AdmissionOperation() && rule.IsMutateExisting() {
						policyContext := policyContext.Copy()
						if err := policyContext.JSONContext().AddTargetResource(patchedResource.unstructured.Object); err != nil {
							logger.Error(err, "failed to add target resource to the context")
							continue
						}
					}

					logger.V(4).Info("apply rule to resource", "resource namespace", patchedResource.unstructured.GetNamespace(), "resource name", patchedResource.unstructured.GetName())
					var mutateResp *mutate.Response
					if rule.Mutation.ForEachMutation != nil {
						m := &forEachMutator{
							rule:          ruleCopy,
							foreach:       rule.Mutation.ForEachMutation,
							policyContext: policyContext,
							resource:      patchedResource,
							log:           logger,
							contextLoader: e.ContextLoader(policyContext.Policy(), *ruleCopy),
							nesting:       0,
						}

						mutateResp = m.mutateForEach(ctx)
					} else {
						mutateResp = mutateResource(ruleCopy, policyContext, patchedResource.unstructured, logger)
					}

					matchedResource = mutateResp.PatchedResource
					logger.Info("matchedResource", "matchedResource", matchedResource)
					if ruleResponse := buildRuleResponse(ruleCopy, mutateResp, patchedResource); ruleResponse != nil {
						internal.AddRuleResponse(&resp.PolicyResponse, ruleResponse, startTime)
					}
				}
			},
		)
		if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.Stats.RulesAppliedCount > 0 {
			break
		}
	}

	for _, r := range resp.PolicyResponse.Rules {
		for _, n := range skippedRules {
			if r.Name == n {
				r.Status = engineapi.RuleStatusSkip
				logger.V(4).Info("rule Status set as skip", "rule skippedRules", r.Name)
			}
		}
	}

	resp.PatchedResource = matchedResource
	return resp
}

func mutateResource(rule *kyvernov1.Rule, ctx engineapi.PolicyContext, resource unstructured.Unstructured, logger logr.Logger) *mutate.Response {
	preconditionsPassed, err := internal.CheckPreconditions(logger, ctx, rule.GetAnyAllConditions())
	if err != nil {
		return mutate.NewErrorResponse("failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		return mutate.NewResponse(engineapi.RuleStatusSkip, resource, nil, "preconditions not met")
	}

	return mutate.Mutate(rule, ctx.JSONContext(), resource, logger)
}

type forEachMutator struct {
	rule          *kyvernov1.Rule
	policyContext engineapi.PolicyContext
	foreach       []kyvernov1.ForEachMutation
	resource      resourceInfo
	nesting       int
	contextLoader engineapi.EngineContextLoader
	log           logr.Logger
}

func (f *forEachMutator) mutateForEach(ctx context.Context) *mutate.Response {
	var applyCount int
	allPatches := make([][]byte, 0)

	for _, foreach := range f.foreach {
		if err := f.contextLoader(ctx, f.rule.Context, f.policyContext.JSONContext()); err != nil {
			f.log.Error(err, "failed to load context")
			return mutate.NewErrorResponse("failed to load context", err)
		}

		preconditionsPassed, err := internal.CheckPreconditions(f.log, f.policyContext, f.rule.GetAnyAllConditions())
		if err != nil {
			return mutate.NewErrorResponse("failed to evaluate preconditions", err)
		}

		if !preconditionsPassed {
			return mutate.NewResponse(engineapi.RuleStatusSkip, f.resource.unstructured, nil, "preconditions not met")
		}

		elements, err := evaluateList(foreach.List, f.policyContext.JSONContext())
		if err != nil {
			msg := fmt.Sprintf("failed to evaluate list %s: %v", foreach.List, err)
			return mutate.NewErrorResponse(msg, err)
		}

		mutateResp := f.mutateElements(ctx, foreach, elements)
		if mutateResp.Status == engineapi.RuleStatusError {
			return mutate.NewErrorResponse("failed to mutate elements", err)
		}

		if mutateResp.Status != engineapi.RuleStatusSkip {
			applyCount++
			if len(mutateResp.Patches) > 0 {
				f.resource.unstructured = mutateResp.PatchedResource
				allPatches = append(allPatches, mutateResp.Patches...)
				if f.resource.unstructured.Object != nil {
					if err := f.policyContext.JSONContext().AddResource(f.resource.unstructured.Object); err != nil {
						f.log.Error(err, "unable to update resource object")
					}
				}
			}
		}
	}
	msg := fmt.Sprintf("%d elements processed", applyCount)
	if applyCount == 0 {
		return mutate.NewResponse(engineapi.RuleStatusSkip, f.resource.unstructured, allPatches, msg)
	}
	return mutate.NewResponse(engineapi.RuleStatusPass, f.resource.unstructured, allPatches, msg)
}

func (f *forEachMutator) mutateElements(ctx context.Context, foreach kyvernov1.ForEachMutation, elements []interface{}) *mutate.Response {
	f.policyContext.JSONContext().Checkpoint()
	defer f.policyContext.JSONContext().Restore()

	patchedResource := f.resource
	var allPatches [][]byte
	if foreach.RawPatchStrategicMerge != nil {
		invertedElement(elements)
	}

	for index, element := range elements {
		if element == nil {
			continue
		}

		f.policyContext.JSONContext().Reset()
		policyContext := f.policyContext.Copy()

		falseVar := false
		if err := addElementToContext(policyContext, element, index, f.nesting, &falseVar); err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to add element to mutate.foreach[%d].context", index), err)
		}

		if err := f.contextLoader(ctx, foreach.Context, policyContext.JSONContext()); err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to load to mutate.foreach[%d].context", index), err)
		}

		preconditionsPassed, err := internal.CheckPreconditions(f.log, policyContext, foreach.AnyAllConditions)
		if err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to evaluate mutate.foreach[%d].preconditions", index), err)
		}

		if !preconditionsPassed {
			f.log.Info("mutate.foreach.preconditions not met", "elementIndex", index)
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
				contextLoader: f.contextLoader,
			}

			mutateResp = m.mutateForEach(ctx)
		} else {
			mutateResp = mutate.ForEach(f.rule.Name, foreach, policyContext.JSONContext(), patchedResource.unstructured, f.log)
		}

		if mutateResp.Status == engineapi.RuleStatusFail || mutateResp.Status == engineapi.RuleStatusError {
			return mutateResp
		}

		if len(mutateResp.Patches) > 0 {
			patchedResource.unstructured = mutateResp.PatchedResource
			allPatches = append(allPatches, mutateResp.Patches...)
		}
	}

	return mutate.NewResponse(engineapi.RuleStatusPass, patchedResource.unstructured, allPatches, "")
}

func buildRuleResponse(rule *kyvernov1.Rule, mutateResp *mutate.Response, info resourceInfo) *engineapi.RuleResponse {
	resp := internal.RuleResponse(*rule, engineapi.Mutation, mutateResp.Message, mutateResp.Status)
	if resp.Status == engineapi.RuleStatusPass {
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

func startMutateResultResponse(resp *engineapi.EngineResponse, policy kyvernov1.PolicyInterface, resource unstructured.Unstructured) {
	if resp == nil {
		return
	}
}

func endMutateResultResponse(logger logr.Logger, resp *engineapi.EngineResponse, startTime time.Time) {
	if resp == nil {
		return
	}
	resp.PolicyResponse.Stats.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.Stats.Timestamp = startTime.Unix()
	logger.V(5).Info("finished processing policy", "processingTime", resp.PolicyResponse.Stats.ProcessingTime.String(), "mutationRulesApplied", resp.PolicyResponse.Stats.RulesAppliedCount)
}
