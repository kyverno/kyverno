package engine

import (
	"context"
)

type Provider[T any] interface {
	Fetch(context.Context) ([]T, error)
}
