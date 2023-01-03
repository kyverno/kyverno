package api

import (
	"context"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type Engine interface {
	Validate(ctx context.Context, rclient registryclient.Client, policyContext *PolicyContext, cfg config.Configuration) *EngineResponse
	Mutate(ctx context.Context, rclient registryclient.Client, policyContext *PolicyContext) *EngineResponse
}
