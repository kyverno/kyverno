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
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/kyverno/kyverno/pkg/utils/api"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func doMutate(
	ctx context.Context,
	client dclient.Interface,
	contextLoader engineapi.ContextLoaderFactory,
	selector engineapi.PolicyExceptionSelector,
	policyContext engineapi.PolicyContext,
	cfg config.Configuration,
) (resp *engineapi.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.Policy()
	resp = &engineapi.EngineResponse{
		Policy: policy,
	}
	matchedResource := policyContext.NewResource()
	enginectx := policyContext.JSONContext()
	var skippedRules []string

	logger := logging.WithName("EngineMutate").WithValues("policy", policy.GetName(), "kind", matchedResource.GetKind(),
		"namespace", matchedResource.GetNamespace(), "name", matchedResource.GetName())

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
				logger := logger.WithValues("rule", rule.Name)
				var excludeResource []string
				if len(cfg.GetExcludeGroupRole()) > 0 {
					excludeResource = cfg.GetExcludeGroupRole()
				}

				kindsInPolicy := append(rule.MatchResources.GetKinds(), rule.ExcludeResources.GetKinds()...)
				subresourceGVKToAPIResource := GetSubresourceGVKToAPIResourceMap(client, kindsInPolicy, policyContext)
				if err = MatchesResourceDescription(subresourceGVKToAPIResource, matchedResource, rule, policyContext.AdmissionInfo(), excludeResource, policyContext.NamespaceLabels(), policyContext.Policy().GetNamespace(), policyContext.SubResource()); err != nil {
					logger.V(4).Info("rule not matched", "reason", err.Error())
					skippedRules = append(skippedRules, rule.Name)
					return
				}

				// check if there is a corresponding policy exception
				ruleResp := hasPolicyExceptions(logger, selector, policyContext, &computeRules[i], subresourceGVKToAPIResource, cfg)
				if ruleResp != nil {
					resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
					return
				}

				logger.V(3).Info("processing mutate rule", "applyRules", applyRules)
				resource, err := policyContext.JSONContext().Query("request.object")
				policyContext.JSONContext().Reset()
				if err == nil && resource != nil {
					if err := enginectx.AddResource(resource.(map[string]interface{})); err != nil {
						logger.Error(err, "unable to update resource object")
					}
				} else {
					logger.Error(err, "failed to query resource object")
				}

				if err := LoadContext(ctx, contextLoader, rule.Context, policyContext, rule.Name); err != nil {
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
					targets, err := loadTargets(client, ruleCopy.Mutation.Targets, policyContext, logger)
					if err != nil {
						rr := ruleResponse(rule, engineapi.Mutation, err.Error(), engineapi.RuleStatusError)
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
							contextLoader: contextLoader,
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
						if ruleResponse.Status == engineapi.RuleStatusError {
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
				r.Status = engineapi.RuleStatusSkip
				logger.V(4).Info("rule Status set as skip", "rule skippedRules", r.Name)
			}
		}
	}

	resp.PatchedResource = matchedResource
	return resp
}

func mutateResource(rule *kyvernov1.Rule, ctx engineapi.PolicyContext, resource unstructured.Unstructured, logger logr.Logger) *mutate.Response {
	preconditionsPassed, err := checkPreconditions(logger, ctx, rule.GetAnyAllConditions())
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
	contextLoader engineapi.ContextLoaderFactory
	log           logr.Logger
}

func (f *forEachMutator) mutateForEach(ctx context.Context) *mutate.Response {
	var applyCount int
	allPatches := make([][]byte, 0)

	for _, foreach := range f.foreach {
		if err := LoadContext(ctx, f.contextLoader, f.rule.Context, f.policyContext, f.rule.Name); err != nil {
			f.log.Error(err, "failed to load context")
			return mutate.NewErrorResponse("failed to load context", err)
		}

		preconditionsPassed, err := checkPreconditions(f.log, f.policyContext, f.rule.GetAnyAllConditions())
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

		if err := LoadContext(ctx, f.contextLoader, foreach.Context, policyContext, f.rule.Name); err != nil {
			return mutate.NewErrorResponse(fmt.Sprintf("failed to load to mutate.foreach[%d].context", index), err)
		}

		preconditionsPassed, err := checkPreconditions(f.log, policyContext, foreach.AnyAllConditions)
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
	resp := ruleResponse(*rule, engineapi.Mutation, mutateResp.Message, mutateResp.Status)
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

	resp.PolicyResponse.Policy.Name = policy.GetName()
	resp.PolicyResponse.Policy.Namespace = policy.GetNamespace()
	resp.PolicyResponse.Resource.Name = resource.GetName()
	resp.PolicyResponse.Resource.Namespace = resource.GetNamespace()
	resp.PolicyResponse.Resource.Kind = resource.GetKind()
	resp.PolicyResponse.Resource.APIVersion = resource.GetAPIVersion()
}

func endMutateResultResponse(logger logr.Logger, resp *engineapi.EngineResponse, startTime time.Time) {
	if resp == nil {
		return
	}

	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.Timestamp = startTime.Unix()
	logger.V(5).Info("finished processing policy", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "mutationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
}
