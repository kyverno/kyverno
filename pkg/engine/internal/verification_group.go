package internal

import (
	"context"

	"golang.org/x/sync/singleflight"
)

type verificationGroup[T any] struct {
	group singleflight.Group
}

func (g *verificationGroup[T]) Do(ctx context.Context, key string, fn func() T) (T, error) {
	result := g.group.DoChan(key, func() (any, error) {
		return fn(), nil
	})
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case value := <-result:
		if value.Err != nil {
			var zero T
			return zero, value.Err
		}
		return value.Val.(T), nil
	}
}
