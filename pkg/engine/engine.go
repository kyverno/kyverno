package engine

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/handlers/manifest"
	"github.com/kyverno/kyverno/pkg/engine/handlers/mutation"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type engine struct {
	configuration         config.Configuration
	client                dclient.Interface
	rclient               registryclient.Client
	contextLoader         engineapi.ContextLoaderFactory
	exceptionSelector     engineapi.PolicyExceptionSelector
	verifyManifestHandler handlers.Handler
	mutateHandler         handlers.Handler
	mutateExistingHandler handlers.Handler
}

func NewEngine(
	configuration config.Configuration,
	client dclient.Interface,
	rclient registryclient.Client,
	contextLoader engineapi.ContextLoaderFactory,
	exceptionSelector engineapi.PolicyExceptionSelector,
) engineapi.Engine {
	e := &engine{
		configuration:         configuration,
		client:                client,
		rclient:               rclient,
		contextLoader:         contextLoader,
		exceptionSelector:     exceptionSelector,
		verifyManifestHandler: manifest.NewHandler(client),
	}
	e.mutateHandler = mutation.NewHandler(configuration, e.ContextLoader)
	e.mutateExistingHandler = mutation.NewMutateExistingHandler(configuration, client, e.ContextLoader)
	return e
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
	loader := e.contextLoader(policy, rule)
	return func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return loader.Load(
			ctx,
			e.client,
			e.rclient,
			contextEntries,
			jsonContext,
		)
	}
}
