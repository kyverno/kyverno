package mutation

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mutateExistingHandler struct {
	client        dclient.Interface
	contextLoader engineapi.EngineContextLoaderFactory
}

func NewMutateExistingHandler(
	client dclient.Interface,
	contextLoader engineapi.EngineContextLoaderFactory,
) handlers.Handler {
	return mutateExistingHandler{
		client:        client,
		contextLoader: contextLoader,
	}
}

func (h mutateExistingHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	startTime := time.Now()
	targets, err := loadTargets(h.client, rule.Mutation.Targets, policyContext, logger)
	if err != nil {
		return resource, handlers.WithError(startTime, rule, engineapi.Mutation, "failed to load targets", err)
	}
	policy := policyContext.Policy()
	contextLoader := h.contextLoader(policy, rule)
	var responses []engineapi.RuleResponse
	for _, patchedResource := range targets {
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
