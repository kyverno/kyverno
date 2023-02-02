package api

import (
	"context"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type Engine interface {
	// Validate applies validation rules from policy on the resource
	Validate(
		ctx context.Context,
		contextLoader ContextLoaderFactory,
		policyContext PolicyContext,
		cfg config.Configuration,
	) *EngineResponse

	// Mutate performs mutation. Overlay first and then mutation patches
	Mutate(
		ctx context.Context,
		contextLoader ContextLoaderFactory,
		policyContext PolicyContext,
	) *EngineResponse

	// VerifyAndPatchImages ...
	VerifyAndPatchImages(
		ctx context.Context,
		contextLoader ContextLoaderFactory,
		rclient registryclient.Client,
		policyContext PolicyContext,
		cfg config.Configuration,
	) (*EngineResponse, *ImageVerificationMetadata)
}
