package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type breakerMetrics struct {
	drops metric.Int64Counter
	total metric.Int64Counter

	logger logr.Logger
}

type BreakerMetrics interface {
	RecordTotalIncrease(ctx context.Context, name string)
	RecordDrop(ctx context.Context, name string)
}

func (m *breakerMetrics) init(meterProvider metric.MeterProvider) {
	var err error
	meter := meterProvider.Meter(MeterName)

	m.drops, err = meter.Int64Counter(
		"kyverno_breaker_drops",
		metric.WithDescription("track the number of times the breaker failed open and dropped"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_breaker_drops")
	}
	m.total, err = meter.Int64Counter(
		"kyverno_breaker_total",
		metric.WithDescription("track number of times the breaker was invoked"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_breaker_total")
	}
}

func (m *breakerMetrics) RecordTotalIncrease(ctx context.Context, breakerName string) {
	m.total.Add(ctx, 1, metric.WithAttributes(attribute.String("circuit_name", breakerName)))
}

func (m *breakerMetrics) RecordDrop(ctx context.Context, breakerName string) {
	m.drops.Add(ctx, 1, metric.WithAttributes(attribute.String("circuit_name", breakerName)))
}
