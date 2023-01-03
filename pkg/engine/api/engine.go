package api

import (
	"context"
)

type Engine interface {
	Validate(context.Context, *PolicyContext) *EngineResponse
	Mutate(context.Context, *PolicyContext) *EngineResponse
	VerifyAndPatchImages(context.Context, *PolicyContext) (*EngineResponse, *ImageVerificationMetadata)
}
