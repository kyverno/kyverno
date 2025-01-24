package breaker

import (
	"context"

	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/metric"
)

type Breaker interface {
	Do(context.Context, func(context.Context) error) error
}

type breaker struct {
	name  string
	drops sdkmetric.Int64Counter
	total sdkmetric.Int64Counter
	open  func(context.Context) bool
}

func NewBreaker(name string, open func(context.Context) bool) *breaker {
	logger := logging.WithName("circuit-breaker")
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	drops, err := meter.Int64Counter(
		"kyverno_breaker_drops",
		sdkmetric.WithDescription("track the number of times the breaker failed open and dropped"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_breaker_drops")
	}
	total, err := meter.Int64Counter(
		"kyverno_breaker_total",
		sdkmetric.WithDescription("track number of times the breaker was invoked"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_breaker_total")
	}
	return &breaker{
		name:  name,
		drops: drops,
		total: total,
		open:  open,
	}
}

func (b *breaker) Do(ctx context.Context, inner func(context.Context) error) error {
	attributes := sdkmetric.WithAttributes(
		attribute.String("circuit_name", b.name),
	)
	if b.total != nil {
		b.total.Add(ctx, 1, attributes)
	}
	if b.open != nil && b.open(ctx) {
		if b.drops != nil {
			b.drops.Add(ctx, 1, attributes)
		}
		return nil
	}
	if inner == nil {
		return nil
	}
	return inner(ctx)
}
