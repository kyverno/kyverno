package engine

import (
	"context"

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
