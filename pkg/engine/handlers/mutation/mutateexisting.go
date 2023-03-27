package mutation

import (
	"context"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type handlerExisting struct {
	configuration config.Configuration
	client        dclient.Interface
	contextLoader func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.EngineContextLoader
}

func NewMutateExistingHandler(
	configuration config.Configuration,
	client dclient.Interface,
	contextLoader func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.EngineContextLoader,
) handlers.Handler {
	return handlerExisting{
		configuration: configuration,
		client:        client,
		contextLoader: contextLoader,
	}
}

func (h handlerExisting) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	polexFilter func(logr.Logger, engineapi.PolicyContext, kyvernov1.Rule) *engineapi.RuleResponse,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	policy := policyContext.Policy()
	contextLoader := h.contextLoader(policy, rule)
	var responses []engineapi.RuleResponse
	var excludeResource []string
	if len(h.configuration.GetExcludedGroups()) > 0 {
		excludeResource = h.configuration.GetExcludedGroups()
	}
	gvk, subresource := policyContext.ResourceKind()
	if err := engineutils.MatchesResourceDescription(
		resource,
		rule,
		policyContext.AdmissionInfo(),
		excludeResource,
		policyContext.NamespaceLabels(),
		policyContext.Policy().GetNamespace(),
		gvk,
		subresource,
	); err != nil {
		logger.V(4).Info("rule not matched", "reason", err.Error())
		return resource, nil
	}

	// check if there is a corresponding policy exception
	if ruleResp := polexFilter(logger, policyContext, rule); ruleResp != nil {
		return resource, handlers.RuleResponses(ruleResp)
	}

	logger.V(3).Info("processing mutate rule")

	if err := contextLoader(ctx, rule.Context, policyContext.JSONContext()); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			logger.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			logger.Error(err, "failed to load context")
		}
		// TODO: return error ?
		return resource, nil
	}

	var patchedResources []resourceInfo
	targets, err := loadTargets(h.client, rule.Mutation.Targets, policyContext, logger)
	if err != nil {
		rr := internal.RuleError(&rule, engineapi.Mutation, "", err)
		responses = append(responses, *rr)
	} else {
		patchedResources = append(patchedResources, targets...)
	}

	for _, patchedResource := range patchedResources {
		if patchedResource.unstructured.Object == nil {
			continue
		}
		policyContext := policyContext.Copy()
		if err := policyContext.JSONContext().AddTargetResource(patchedResource.unstructured.Object); err != nil {
			logger.Error(err, "failed to add target resource to the context")
			continue
		}

		// logger.V(4).Info("apply rule to resource", "resource namespace", patchedResource.unstructured.GetNamespace(), "resource name", patchedResource.unstructured.GetName())
		var mutateResp *mutate.Response
		if rule.Mutation.ForEachMutation != nil {
			m := &forEachMutator{
				rule:          &rule,
				foreach:       rule.Mutation.ForEachMutation,
				policyContext: policyContext,
				resource:      patchedResource,
				log:           logger,
				contextLoader: contextLoader,
				nesting:       0,
			}
			mutateResp = m.mutateForEach(ctx)
		} else {
			mutateResp = mutateResource(&rule, policyContext, patchedResource.unstructured, logger)
		}
		if ruleResponse := buildRuleResponse(&rule, mutateResp, patchedResource); ruleResponse != nil {
			responses = append(responses, *ruleResponse)
		}
	}
	return resource, responses
}
