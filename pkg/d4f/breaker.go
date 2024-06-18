package d4f

import (
	"context"

	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/metric"
)

type Breaker interface {
	Do(context.Context, func(context.Context) error) error
}

type breaker struct {
	open func(context.Context) bool
}

func NewBreaker(name string, open func(context.Context) bool) *breaker {
	logger := logging.WithName("cricuit-breaker")
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	breakerOpen, err := meter.Int64ObservableGauge(
		"kyverno_circuitbreaker_open",
		sdkmetric.WithDescription("track circuit breakers state"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_controller_reconcile_total")
	}
	b := &breaker{
		open: open,
	}
	if breakerOpen != nil {
		callback := func(ctx context.Context, observer metric.Observer) error {
			value := 0
			if open(ctx) {
				value = 1
			}
			observer.ObserveInt64(
				breakerOpen,
				int64(value),
				metric.WithAttributes(
					attribute.String("name", name),
				),
			)
			return nil
		}
		if _, err := meter.RegisterCallback(callback, breakerOpen); err != nil {
			logger.Error(err, "failed to register callback")
		}
	}
	return b
}

func (b *breaker) Do(ctx context.Context, inner func(context.Context) error) error {
	if b.open(ctx) {
		return nil
	}
	return inner(ctx)
}
