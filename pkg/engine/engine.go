package engine

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type engine struct {
	configuration     config.Configuration
	client            dclient.Interface
	rclient           registryclient.Client
	contextLoader     engineapi.ContextLoaderFactory
	exceptionSelector engineapi.PolicyExceptionSelector
}

func NewEngine(
	configuration config.Configuration,
	client dclient.Interface,
	rclient registryclient.Client,
	contextLoader engineapi.ContextLoaderFactory,
	exceptionSelector engineapi.PolicyExceptionSelector,
) engineapi.Engine {
	return &engine{
		configuration:     configuration,
		client:            client,
		rclient:           rclient,
		contextLoader:     contextLoader,
		exceptionSelector: exceptionSelector,
	}
}

func (e *engine) Validate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.validate"), policyContext)
	return engineapi.EngineResponse{
		// TODO
		// PatchedResource unstructured.Unstructured
		Policy:          policyContext.Policy(),
		NamespaceLabels: policyContext.NamespaceLabels(),
		PolicyResponse:  e.validate(ctx, logger, policyContext),
	}
}

func (e *engine) Mutate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.mutate"), policyContext)
	return engineapi.EngineResponse{
		// TODO
		// PatchedResource unstructured.Unstructured
		Policy:          policyContext.Policy(),
		NamespaceLabels: policyContext.NamespaceLabels(),
		PolicyResponse:  e.mutate(ctx, logger, policyContext),
	}
}

func (e *engine) VerifyAndPatchImages(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) (engineapi.EngineResponse, engineapi.ImageVerificationMetadata) {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.verify"), policyContext)
	return e.verifyAndPatchImages(ctx, logger, policyContext)
}

func (e *engine) ApplyBackgroundChecks(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.background"), policyContext)
	return engineapi.EngineResponse{
		// TODO
		// PatchedResource unstructured.Unstructured
		Policy:          policyContext.Policy(),
		NamespaceLabels: policyContext.NamespaceLabels(),
		PolicyResponse:  e.applyBackgroundChecks(ctx, logger, policyContext),
	}
}

func (e *engine) GenerateResponse(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) engineapi.EngineResponse {
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.generate"), policyContext)
	return engineapi.EngineResponse{
		// TODO
		// PatchedResource unstructured.Unstructured
		Policy:          policyContext.Policy(),
		NamespaceLabels: policyContext.NamespaceLabels(),
		PolicyResponse:  e.generateResponse(ctx, logger, policyContext, gr),
	}
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
