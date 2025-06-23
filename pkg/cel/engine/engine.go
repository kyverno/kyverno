package engine

import (
	"context"
)

type Engine[T any] interface {
	Handle(context.Context, EngineRequest, func(T) bool) (EngineResponse, error)
}
