package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type _engine struct{}

func NewEngine() api.Engine {
	return &_engine{}
}

func (e *_engine) Mutate(ctx context.Context, rclient registryclient.Client, policyContext *api.PolicyContext) *api.EngineResponse {
	return engineMutate(ctx, rclient, policyContext)
}
func (e *_engine) Validate(ctx context.Context, rclient registryclient.Client, policyContext *api.PolicyContext, cfg config.Configuration) *api.EngineResponse {
	return engineValidate(ctx, rclient, policyContext, cfg)
}

func (e *_engine) VerifyAndPatchImages(ctx context.Context, rclient registryclient.Client, policyContext *api.PolicyContext, cfg config.Configuration) (*api.EngineResponse, *api.ImageVerificationMetadata) {
	return verifyAndPatchImages(ctx, rclient, policyContext, cfg)
}
