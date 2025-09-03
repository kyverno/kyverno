package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/metric"
)

func GetEventMetrics() EventMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.EventMetrics()
}

type eventMetrics struct {
	dropped metric.Int64Counter

	logger logr.Logger
}

type EventMetrics interface {
	RecordDrop(ctx context.Context)
}

func (m *eventMetrics) init(meterProvider metric.MeterProvider) {
	var err error
	meter := meterProvider.Meter(MeterName)

	m.dropped, err = meter.Int64Counter(
		"kyverno_events_dropped",
		metric.WithDescription("can be used to track the number of events dropped by the event generator"),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_events_dropped")
	}
}

func (m *eventMetrics) RecordDrop(ctx context.Context) {
	m.dropped.Add(ctx, 1)
}
