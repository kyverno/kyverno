package engine

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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
	policyContext engineapi.PolicyContext,
) (*engineapi.EngineResponse, *engineapi.ImageVerificationMetadata) {
	return e.verifyAndPatchImages(ctx, policyContext)
}

func (e *engine) ApplyBackgroundChecks(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return e.applyBackgroundChecks(policyContext)
}

func (e *engine) GenerateResponse(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) *engineapi.EngineResponse {
	return e.generateResponse(policyContext, gr)
}

func (e *engine) ContextLoader(
	policy kyvernov1.PolicyInterface,
	rule kyvernov1.Rule,
) engineapi.EngineContextLoader {
	return nil
	// return e.contextLoader(policy, rule)
}
