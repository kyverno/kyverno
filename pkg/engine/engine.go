package engine

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type engine struct {
	configuration     config.Configuration
	client            dclient.Interface
	contextLoader     engineapi.ContextLoaderFactory
	exceptionSelector engineapi.PolicyExceptionSelector
}

func NewEngine(
	configuration config.Configuration,
	client dclient.Interface,
	contextLoader engineapi.ContextLoaderFactory,
	exceptionSelector engineapi.PolicyExceptionSelector,
) engineapi.Engine {
	return &engine{
		configuration:     configuration,
		client:            client,
		contextLoader:     contextLoader,
		exceptionSelector: exceptionSelector,
	}
}

func (e *engine) Validate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return e.validate(ctx, policyContext)
}

func (e *engine) Mutate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return e.mutate(ctx, policyContext)
}

func (e *engine) VerifyAndPatchImages(
	ctx context.Context,
	rclient registryclient.Client,
	policyContext engineapi.PolicyContext,
) (*engineapi.EngineResponse, *engineapi.ImageVerificationMetadata) {
	return e.verifyAndPatchImages(ctx, rclient, policyContext)
}

func (e *engine) ApplyBackgroundChecks(
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return e.applyBackgroundChecks(policyContext)
}

func (e *engine) GenerateResponse(
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) *engineapi.EngineResponse {
	return e.generateResponse(policyContext, gr)
}

func (e *engine) ContextLoader(
	policyContext engineapi.PolicyContext,
	ruleName string,
) engineapi.ContextLoader {
	return e.contextLoader(policyContext, ruleName)
}
