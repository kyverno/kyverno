package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type _engine struct {
	dclient dclient.Interface
	rclient registryclient.Client
	config  config.Configuration
	// informerCacheResolvers resolvers.ConfigmapResolver
	// // TODO: create a resolver instead
	// peLister kyvernov2alpha1listers.PolicyExceptionLister
}

func NewEngine(
	dclient dclient.Interface,
	rclient registryclient.Client,
	config config.Configuration,
) api.Engine {
	return &_engine{
		dclient: dclient,
		rclient: rclient,
		config:  config,
	}
}

func (e *_engine) Mutate(ctx context.Context, policyContext *api.PolicyContext) *api.EngineResponse {
	return engineMutate(ctx, e.rclient, policyContext)
}

func (e *_engine) Validate(ctx context.Context, policyContext *api.PolicyContext) *api.EngineResponse {
	return engineValidate(ctx, e.rclient, policyContext, e.config)
}

func (e *_engine) VerifyAndPatchImages(ctx context.Context, policyContext *api.PolicyContext) (*api.EngineResponse, *api.ImageVerificationMetadata) {
	return verifyAndPatchImages(ctx, e.rclient, policyContext, e.config)
}
