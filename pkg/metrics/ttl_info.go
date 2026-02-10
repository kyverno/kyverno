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
	RecordDeletedObject(ctx context.Context, gvr schema.GroupVersionResource, namespace string)
	RecordTTLFailure(ctx context.Context, gvr schema.GroupVersionResource, namespace string)
	RegisterCallback(f metric.Callback) (metric.Registration, error)
}

type ttlInfoMetrics struct {
	deletedObjectsTotal metric.Int64Counter
	ttlFailureTotal     metric.Int64Counter

	infoMetric metric.Int64ObservableGauge
	meter      metric.Meter
	callback   metric.Callback

	logger logr.Logger
}

func (m *ttlInfoMetrics) init(meter metric.Meter) {
	var err error

	m.deletedObjectsTotal, err = meter.Int64Counter(
		"kyverno_ttl_controller_deletedobjects",
		metric.WithDescription("can be used to track number of deleted objects by the ttl resource controller."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, ttl_controller_deletedobjects_total")
	}
	m.ttlFailureTotal, err = meter.Int64Counter(
		"kyverno_ttl_controller_errors",
		metric.WithDescription("can be used to track number of ttl cleanup failures."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, ttl_controller_errors_total")
	}

	m.infoMetric, err = meter.Int64ObservableGauge(
		"kyverno_ttl_controller_info",
		metric.WithDescription("can be used to track individual resource controllers running for ttl based cleanup"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_ttl_controller_info")
	}

	m.meter = meter

	if m.callback != nil {
		if _, err := m.meter.RegisterCallback(m.callback, m.infoMetric); err != nil {
			m.logger.Error(err, "failed to register callback for ttl info metric")
		}
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
	if m.meter == nil {
		return nil, nil
	}

	m.callback = f
	return m.meter.RegisterCallback(f, m.infoMetric)
}

func (m *ttlInfoMetrics) RecordDeletedObject(ctx context.Context, gvr schema.GroupVersionResource, namespace string) {
	if m.deletedObjectsTotal == nil {
		return
	}

	labels := []attribute.KeyValue{
		attribute.String("resource_namespace", namespace),
		attribute.String("resource_group", gvr.Group),
		attribute.String("resource_version", gvr.Version),
		attribute.String("resource_resource", gvr.Resource),
	}

	m.deletedObjectsTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}

func (m *ttlInfoMetrics) RecordTTLFailure(ctx context.Context, gvr schema.GroupVersionResource, namespace string) {
	if m.ttlFailureTotal == nil {
		return
	}

	labels := []attribute.KeyValue{
		attribute.String("resource_namespace", namespace),
		attribute.String("resource_group", gvr.Group),
		attribute.String("resource_version", gvr.Version),
		attribute.String("resource_resource", gvr.Resource),
	}

	m.ttlFailureTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}
