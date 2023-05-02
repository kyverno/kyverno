package mutation

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mutateExistingHandler struct {
	client dclient.Interface
}

func NewMutateExistingHandler(
	client dclient.Interface,
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
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	var responses []engineapi.RuleResponse
	logger.V(3).Info("processing mutate rule")
	targets, err := loadTargets(h.client, rule.Mutation.Targets, policyContext, logger)
	if err != nil {
		rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "", err)
		responses = append(responses, *rr)
	}

	for _, target := range targets {
		if target.unstructured.Object == nil {
			continue
		}
		policyContext := policyContext.Copy()
		if err := policyContext.JSONContext().AddTargetResource(target.unstructured.Object); err != nil {
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
		preconditionsPassed, err := internal.CheckPreconditions(logger, policyContext.JSONContext(), target.preconditions)
		if err != nil {
			rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to evaluate preconditions", err)
			responses = append(responses, *rr)
			continue
		}
		if !preconditionsPassed {
			rr := engineapi.RuleSkip(rule.Name, engineapi.Mutation, "preconditions not met")
			responses = append(responses, *rr)
			continue
		}
		var patchers []patch.Patcher
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
			p, err := m.mutateForEach(ctx)
			if err != nil {
				rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to collect patchers", err)
				responses = append(responses, *rr)
				continue
			}
			patchers = p
		} else {
			p, err := mutate.Mutate(logger, &rule, policyContext.JSONContext())
			if err != nil {
				rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to collect patchers", err)
				responses = append(responses, *rr)
				continue
			}
			patchers = append(patchers, p)
		}
		resource, response := mutate.ApplyPatchers(logger, target.unstructured, rule, patchers...)
		if response != nil {
			err := policyContext.JSONContext().AddTargetResource(resource.Object)
			if err != nil {
				rr := engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to update patched resource in the JSON context", err)
				responses = append(responses, *rr)
				continue
			}
			response = response.WithPatchedTarget(&resource, target.parentResourceGVR, target.subresource)
			responses = append(responses, *response)
		}
	}
	return resource, responses
}
