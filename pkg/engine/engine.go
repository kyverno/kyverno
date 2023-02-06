package engine

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type engine struct {
	configuration     config.Configuration
	contextLoader     engineapi.ContextLoaderFactory
	exceptionSelector engineapi.PolicyExceptionSelector
}

func NewEngine(
	configuration config.Configuration,
	contextLoader engineapi.ContextLoaderFactory,
	exceptionSelector engineapi.PolicyExceptionSelector,
) engineapi.Engine {
	return &engine{
		configuration:     configuration,
		contextLoader:     contextLoader,
		exceptionSelector: exceptionSelector,
	}
}

func (e *engine) Validate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return doValidate(ctx, e.contextLoader, e.exceptionSelector, policyContext, e.configuration)
}

func (e *engine) Mutate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return doMutate(ctx, e.contextLoader, e.exceptionSelector, policyContext, e.configuration)
}

func (e *engine) VerifyAndPatchImages(
	ctx context.Context,
	rclient registryclient.Client,
	policyContext engineapi.PolicyContext,
) (*engineapi.EngineResponse, *engineapi.ImageVerificationMetadata) {
	return doVerifyAndPatchImages(ctx, e.contextLoader, e.exceptionSelector, rclient, policyContext, e.configuration)
}

func (e *engine) ApplyBackgroundChecks(
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return doApplyBackgroundChecks(e.contextLoader, e.exceptionSelector, policyContext, e.configuration)
}

func (e *engine) GenerateResponse(
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) *engineapi.EngineResponse {
	return doGenerateResponse(e.contextLoader, e.exceptionSelector, policyContext, gr, e.configuration)
}

func (e *engine) ContextLoader(
	policyContext engineapi.PolicyContext,
	ruleName string,
) engineapi.ContextLoader {
	return e.contextLoader(policyContext, ruleName)
}
