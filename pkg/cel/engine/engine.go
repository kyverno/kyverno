package engine

import (
	"context"
)

type Engine interface {
	Handle(context.Context, EngineRequest) (EngineResponse, error)
}
