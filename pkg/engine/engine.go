package engine

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/handlers/mutation"
	"github.com/kyverno/kyverno/pkg/engine/handlers/validation"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type engine struct {
	configuration              config.Configuration
	client                     dclient.Interface
	rclient                    registryclient.Client
	engineContextLoaderFactory engineapi.EngineContextLoaderFactory
	exceptionSelector          engineapi.PolicyExceptionSelector
	validateResourceHandler    handlers.Handler
	validateImageHandler       handlers.Handler
	validateManifestHandler    handlers.Handler
	validatePssHandler         handlers.Handler
	mutateResourceHandler      handlers.Handler
	mutateExistingHandler      handlers.Handler
}

func NewEngine(
	configuration config.Configuration,
	client dclient.Interface,
	rclient registryclient.Client,
	contextLoader engineapi.ContextLoaderFactory,
	exceptionSelector engineapi.PolicyExceptionSelector,
) engineapi.Engine {
	engineContextLoaderFactory := func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) engineapi.EngineContextLoader {
		loader := contextLoader(policy, rule)
		return func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
			return loader.Load(
				ctx,
				client,
				rclient,
				contextEntries,
				jsonContext,
			)
		}
	}
	return &engine{
		configuration:              configuration,
		client:                     client,
		rclient:                    rclient,
		engineContextLoaderFactory: engineContextLoaderFactory,
		exceptionSelector:          exceptionSelector,
		validateResourceHandler:    validation.NewValidateResourceHandler(engineContextLoaderFactory),
		validateImageHandler:       validation.NewValidateImageHandler(configuration),
		validateManifestHandler:    validation.NewValidateManifestHandler(client),
		validatePssHandler:         validation.NewValidatePssHandler(),
		mutateResourceHandler:      mutation.NewMutateResourceHandler(engineContextLoaderFactory),
		mutateExistingHandler:      mutation.NewMutateExistingHandler(client, engineContextLoaderFactory),
	}
}

func (e *engine) Validate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.validate"), policyContext)
	if !internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		return engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	}
	return e.validate(ctx, logger, policyContext)
}

func (e *engine) Mutate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.mutate"), policyContext)
	if !internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		return engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	}
	return e.mutate(ctx, logger, policyContext)
}

func (e *engine) VerifyAndPatchImages(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) (engineapi.EngineResponse, engineapi.ImageVerificationMetadata) {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.verify"), policyContext)
	if !internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		return engineapi.NewEngineResponseFromPolicyContext(policyContext, nil), engineapi.ImageVerificationMetadata{}
	}
	return e.verifyAndPatchImages(ctx, logger, policyContext)
}

func (e *engine) ApplyBackgroundChecks(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.background"), policyContext)
	if !internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		return engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	}
	return e.applyBackgroundChecks(ctx, logger, policyContext)
}

func (e *engine) GenerateResponse(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.generate"), policyContext)
	if !internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		return engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	}
	return e.generateResponse(ctx, logger, policyContext, gr)
}

func (e *engine) ContextLoader(
	policy kyvernov1.PolicyInterface,
	rule kyvernov1.Rule,
) engineapi.EngineContextLoader {
	return e.engineContextLoaderFactory(policy, rule)
}

// matches checks if either the new or old resource satisfies the filter conditions defined in the rule
func matches(
	rule kyvernov1.Rule,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
) error {
	gvk, subresource := policyContext.ResourceKind()
	err := engineutils.MatchesResourceDescription(
		resource,
		rule,
		policyContext.AdmissionInfo(),
		policyContext.NamespaceLabels(),
		policyContext.Policy().GetNamespace(),
		gvk,
		subresource,
		policyContext.Operation(),
	)
	if err == nil {
		return nil
	}
	oldResource := policyContext.OldResource()
	if oldResource.Object != nil {
		err := engineutils.MatchesResourceDescription(
			policyContext.OldResource(),
			rule,
			policyContext.AdmissionInfo(),
			policyContext.NamespaceLabels(),
			policyContext.Policy().GetNamespace(),
			gvk,
			subresource,
			policyContext.Operation(),
		)
		if err == nil {
			return nil
		}
	}
	return err
}

func (e *engine) invokeRuleHandler(
	ctx context.Context,
	logger logr.Logger,
	handler handlers.Handler,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	ruleType engineapi.RuleType,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	return tracing.ChildSpan2(
		ctx,
		"pkg/engine",
		fmt.Sprintf("RULE %s", rule.Name),
		func(ctx context.Context, span trace.Span) (unstructured.Unstructured, []engineapi.RuleResponse) {
			// check if resource and rule match
			if err := matches(rule, policyContext, resource); err != nil {
				logger.V(4).Info("rule not matched", "reason", err.Error())
				return resource, nil
			}
			// check if there's an exception
			if ruleResp := e.hasPolicyExceptions(logger, ruleType, policyContext, rule); ruleResp != nil {
				return resource, handlers.RuleResponses(ruleResp)
			}
			// load rule context
			if err := internal.LoadContext(ctx, e, policyContext, rule); err != nil {
				if _, ok := err.(gojmespath.NotFoundError); ok {
					logger.V(3).Info("failed to load context", "reason", err.Error())
				} else {
					logger.Error(err, "failed to load context")
				}
				// TODO: return error ?
				return resource, nil
			}
			// check preconditions
			preconditionsPassed, err := internal.CheckPreconditions(logger, policyContext, rule.GetAnyAllConditions())
			if err != nil {
				return resource, handlers.RuleResponses(internal.RuleError(&rule, ruleType, "failed to evaluate preconditions", err))
			}
			if !preconditionsPassed {
				return resource, handlers.RuleResponses(internal.RuleSkip(&rule, ruleType, "preconditions not met"))
			}
			// process handler
			return handler.Process(ctx, logger, policyContext, resource, rule)
		},
	)
}
