package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ControllerMetrics interface {
	RecordReconcileIncrease(ctx context.Context, controllerName string)
	RecordRequeueIncrease(ctx context.Context, controllerName string, requeues int)
	RecordQueueDrop(ctx context.Context, controllerName string)
}

type controllerMetrics struct {
	reconcileTotal metric.Int64Counter
	requeueTotal   metric.Int64Counter
	queueDropTotal metric.Int64Counter

	logger logr.Logger
}

func (m *controllerMetrics) init(meterProvider metric.MeterProvider) {
	var err error
	meter := meterProvider.Meter(MeterName)

	m.reconcileTotal, err = meter.Int64Counter(
		"kyverno_controller_reconcile",
		metric.WithDescription("can be used to track number of reconciliation cycles"))
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_controller_reconcile_total")
	}
	m.requeueTotal, err = meter.Int64Counter(
		"kyverno_controller_requeue",
		metric.WithDescription("can be used to track number of reconciliation errors"))
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_controller_requeue_total")
	}
	m.queueDropTotal, err = meter.Int64Counter(
		"kyverno_controller_drop",
		metric.WithDescription("can be used to track number of queue drops"))
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_controller_drop_total")
	}
}

func (m *controllerMetrics) RecordReconcileIncrease(ctx context.Context, controllerName string) {
	m.reconcileTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("controller_name", controllerName)))
}

func (m *controllerMetrics) RecordRequeueIncrease(ctx context.Context, controllerName string, requeues int) {
	m.requeueTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("controller_name", controllerName), attribute.Int("num_requeues", requeues)))
}

func (m *controllerMetrics) RecordQueueDrop(ctx context.Context, controllerName string) {
	m.queueDropTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("controller_name", controllerName)))
}
