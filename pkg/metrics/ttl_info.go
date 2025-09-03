package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetTTLInfoMetrics() TTLInfoMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.TTLInfoMetrics()
}

type TTLInfoMetrics interface {
	RecordTTLInfo(ctx context.Context, gvr schema.GroupVersionResource, observer metric.Observer)
	RegisterCallback(f metric.Callback) (metric.Registration, error)
}

type ttlInfoMetrics struct {
	infoMetric metric.Int64ObservableGauge
	meter      metric.Meter
	callback   metric.Callback

	logger logr.Logger
}

func (m *ttlInfoMetrics) init(meterProvider metric.MeterProvider) {
	var err error
	meter := meterProvider.Meter(MeterName)

	m.infoMetric, err = meter.Int64ObservableGauge(
		"kyverno_ttl_controller_info",
		metric.WithDescription("can be used to track individual resource controllers running for ttl based cleanup"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_ttl_controller_info")
	}

	m.meter = meter

	if m.callback != nil {
		meter.RegisterCallback(m.callback, m.infoMetric)
	}
}

func (m *ttlInfoMetrics) RecordTTLInfo(ctx context.Context, gvr schema.GroupVersionResource, observer metric.Observer) {
	observer.ObserveInt64(m.infoMetric, 1, metric.WithAttributes(
		attribute.String("resource_group", gvr.Group),
		attribute.String("resource_version", gvr.Version),
		attribute.String("resource_resource", gvr.Resource),
	))
}

func (m *ttlInfoMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	m.callback = f
	return m.meter.RegisterCallback(f, m.infoMetric)
}
