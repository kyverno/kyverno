package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Provider[T any] interface {
	Fetch(context.Context) ([]T, error)
}

type TracingProvider[T any] struct {
	inner Provider[T]
}

func (p *TracingProvider[T]) Fetch(ctx context.Context) ([]T, error) {
	return tracing.Span2(ctx, "", "provider/fetch", func(ctx context.Context, s trace.Span) ([]T, error) {
		policies, err := p.inner.Fetch(ctx)
		if err != nil {
			return nil, err
		}

		s.SetAttributes(attribute.Key("policies.count").Int(len(policies)))

		return policies, nil
	})
}

func ProviderWithTrace[T any](provider Provider[T]) Provider[T] {
	return &TracingProvider[T]{inner: provider}
}
