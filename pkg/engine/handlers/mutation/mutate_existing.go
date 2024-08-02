package mutation

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	stringutils "github.com/kyverno/kyverno/pkg/utils/strings"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type mutateExistingHandler struct {
	client engineapi.Client
}

func NewMutateExistingHandler(
	client engineapi.Client,
) (handlers.Handler, error) {
	return mutateExistingHandler{
		client: client,
	}, nil
}

func (h mutateExistingHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
	exceptions []kyvernov2beta1.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there is a policy exception matches the incoming resource
	exception := engineutils.MatchesException(exceptions, policyContext, logger)
	if exception != nil {
		key, err := cache.MetaNamespaceKeyFunc(exception)
		if err != nil {
			logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
			return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
		} else {
			logger.V(3).Info("policy rule skipped due to policy exception", "exception", key)
			return resource, handlers.WithResponses(
				engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule skipped due to policy exception "+key).WithException(exception),
			)
		}
	}

	var responses []engineapi.RuleResponse
	logger.V(3).Info("processing mutate rule")
	targets, err := loadTargets(ctx, h.client, rule.Mutation.Targets, policyContext, logger)
	if err != nil {
		rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "", err)
		responses = append(responses, *rr)
	}

	for _, target := range targets {
		if target.unstructured.Object == nil {
			continue
		}
		policyContext := policyContext
		if err := policyContext.JSONContext().SetTargetResource(target.unstructured.Object); err != nil {
			logger.Error(err, "failed to add target resource to the context")
			continue
		}
		// load target specific context
		if err := contextLoader(ctx, target.context, policyContext.JSONContext()); err != nil {
			rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to load context", err)
			responses = append(responses, *rr)
			continue
		}
		// load target specific preconditions
		preconditionsPassed, msg, err := internal.CheckPreconditions(logger, policyContext.JSONContext(), target.preconditions)
		if err != nil {
			rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to evaluate preconditions", err)
			responses = append(responses, *rr)
			continue
		}
		if !preconditionsPassed {
			s := stringutils.JoinNonEmpty([]string{"preconditions not met", msg}, "; ")
			rr := engineapi.RuleSkip(rule.Name, engineapi.Mutation, s)
			responses = append(responses, *rr)
			continue
		}

		// logger.V(4).Info("apply rule to resource", "resource namespace", patchedResource.unstructured.GetNamespace(), "resource name", patchedResource.unstructured.GetName())
		var mutateResp *mutate.Response
		if rule.Mutation.ForEachMutation != nil {
			m := &forEachMutator{
				rule:          rule,
				foreach:       rule.Mutation.ForEachMutation,
				policyContext: policyContext,
				resource:      target.resourceInfo,
				logger:        logger,
				contextLoader: contextLoader,
				nesting:       0,
			}
			mutateResp = m.mutateForEach(ctx)
		} else {
			mutateResp = mutate.Mutate(&rule, policyContext.JSONContext(), target.unstructured, logger)
		}
		if ruleResponse := buildRuleResponse(&rule, mutateResp, target.resourceInfo); ruleResponse != nil {
			responses = append(responses, *ruleResponse)
		}
	}
	return resource, responses
}
