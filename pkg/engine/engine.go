package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type engine struct{}

func NewEgine() engineapi.Engine {
	return &engine{}
}

func (e *engine) Validate(
	ctx context.Context,
	contextLoader engineapi.ContextLoaderFactory,
	policyContext engineapi.PolicyContext,
	cfg config.Configuration,
) *engineapi.EngineResponse {
	return doValidate(ctx, contextLoader, policyContext, cfg)
}

func (e *engine) Mutate(
	ctx context.Context,
	contextLoader engineapi.ContextLoaderFactory,
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	return doMutate(ctx, contextLoader, policyContext)
}

func (e *engine) VerifyAndPatchImages(
	ctx context.Context,
	contextLoader engineapi.ContextLoaderFactory,
	rclient registryclient.Client,
	policyContext engineapi.PolicyContext,
	cfg config.Configuration,
) (*engineapi.EngineResponse, *engineapi.ImageVerificationMetadata) {
	return doVerifyAndPatchImages(ctx, contextLoader, rclient, policyContext, cfg)
}
