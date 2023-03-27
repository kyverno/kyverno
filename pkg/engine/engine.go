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
	validateManifestHandler    handlers.Handler
	mutateResourceHandler      handlers.Handler
	mutateExistingHandler      handlers.Handler
	validateResourceHandler    handlers.Handler
	validateImageHandler       handlers.Handler
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
		validateManifestHandler:    validation.NewValidateManifestHandler(client),
		validateImageHandler:       validation.NewValidateImageHandler(configuration),
		validateResourceHandler:    validation.NewValidateResourceHandler(engineContextLoaderFactory),
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

func (e *engine) invokeRuleHandler(
	ctx context.Context,
	logger logr.Logger,
	handler handlers.Handler,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	polexFilter func(logr.Logger, engineapi.PolicyContext, kyvernov1.Rule) *engineapi.RuleResponse,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	return tracing.ChildSpan2(
		ctx,
		"pkg/engine",
		fmt.Sprintf("RULE %s", rule.Name),
		func(ctx context.Context, span trace.Span) (unstructured.Unstructured, []engineapi.RuleResponse) {
			// check if resource and rule match
			var excludeResource []string
			if len(e.configuration.GetExcludedGroups()) > 0 {
				excludeResource = e.configuration.GetExcludedGroups()
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
			// check if there's an exception
			if ruleResp := polexFilter(logger, policyContext, rule); ruleResp != nil {
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
			// process handler
			return handler.Process(ctx, logger, policyContext, resource, rule)
		},
	)
}
