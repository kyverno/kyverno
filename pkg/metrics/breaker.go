package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetBreakerMetrics() BreakerMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.BreakerMetrics()
}

type breakerMetrics struct {
	drops metric.Int64Counter
	total metric.Int64Counter

	logger logr.Logger
}

type BreakerMetrics interface {
	RecordTotalIncrease(ctx context.Context, name string)
	RecordDrop(ctx context.Context, name string)
}

func (m *breakerMetrics) init(meter metric.Meter) {
	var err error

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
	if m.total == nil {
		return
	}

	m.total.Add(ctx, 1, metric.WithAttributes(attribute.String("circuit_name", breakerName)))
}

func (m *breakerMetrics) RecordDrop(ctx context.Context, breakerName string) {
	if m.drops == nil {
		return
	}

	m.drops.Add(ctx, 1, metric.WithAttributes(attribute.String("circuit_name", breakerName)))
}
