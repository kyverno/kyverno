package api

import (
	"context"

	"github.com/kyverno/kyverno/pkg/registryclient"
)

type Engine interface {
	Mutate(ctx context.Context, rclient registryclient.Client, policyContext *PolicyContext) (resp *EngineResponse)
}
