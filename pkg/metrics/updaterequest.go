package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetUpdateRequestMetrics() UpdateRequestMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.UpdateRequestMetrics()
}

type UpdateRequestMetrics interface {
	RecordTotal(ctx context.Context, namespace string, total int64, observer metric.Observer)
	RegisterCallback(f metric.Callback) (metric.Registration, error)
}

type updateRequestMetrics struct {
	totalMetric metric.Int64ObservableGauge
	meter       metric.Meter
	callback    metric.Callback

	logger logr.Logger
}

func (m *updateRequestMetrics) init(meter metric.Meter) {
	var err error

	m.totalMetric, err = meter.Int64ObservableGauge(
		"kyverno_updaterequest_total",
		metric.WithDescription("can be used to track the current number of updaterequests in the cluster"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_updaterequest_total")
	}

	m.meter = meter

	if m.callback != nil {
		if _, err := m.meter.RegisterCallback(m.callback, m.totalMetric); err != nil {
			m.logger.Error(err, "failed to register callback for update request total metric")
		}
	}
}

func (m *updateRequestMetrics) RecordTotal(ctx context.Context, namespace string, total int64, observer metric.Observer) {
	if m.totalMetric == nil {
		return
	}

	observer.ObserveInt64(m.totalMetric, total, metric.WithAttributes(
		attribute.String("resource_namespace", namespace),
	))
}

func (m *updateRequestMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	if m.meter == nil {
		return nil, nil
	}

	m.callback = f
	return m.meter.RegisterCallback(f, m.totalMetric)
}
