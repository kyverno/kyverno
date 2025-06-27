package mutation

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
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
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there are policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesException(exceptions, policyContext, logger)
	if len(matchedExceptions) > 0 {
		exceptions := make([]engineapi.GenericException, 0, len(matchedExceptions))
		var keys []string
		for i, exception := range matchedExceptions {
			key, err := cache.MetaNamespaceKeyFunc(&matchedExceptions[i])
			if err != nil {
				logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
				return resource, handlers.WithError(rule, engineapi.Mutation, "failed to compute exception key", err)
			}
			keys = append(keys, key)
			exceptions = append(exceptions, engineapi.NewPolicyException(&exception))
		}

		logger.V(3).Info("policy rule is skipped due to policy exceptions", "exceptions", keys)
		return resource, handlers.WithResponses(
			engineapi.RuleSkip(rule.Name, engineapi.Mutation, "rule is skipped due to policy exceptions"+strings.Join(keys, ", "), rule.ReportProperties).WithExceptions(exceptions),
		)
	}

	var responses []engineapi.RuleResponse
	logger.V(3).Info("processing mutate rule")
	targets, err := loadTargets(ctx, h.client, rule.Mutation.Targets, policyContext, logger)
	if err != nil {
		rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "", err, rule.ReportProperties)
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
			rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to load context", err, rule.ReportProperties)
			responses = append(responses, *rr)
			continue
		}
		// load target specific preconditions
		preconditionsPassed, msg, err := internal.CheckPreconditions(logger, policyContext.JSONContext(), target.preconditions)
		if err != nil {
			rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to evaluate preconditions", err, rule.ReportProperties)
			responses = append(responses, *rr)
			continue
		}
		if !preconditionsPassed {
			s := stringutils.JoinNonEmpty([]string{"preconditions not met", msg}, "; ")
			rr := engineapi.RuleSkip(rule.Name, engineapi.Mutation, s, rule.ReportProperties)
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
