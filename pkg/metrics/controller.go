package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetControllerMetrics() ControllerMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.ControllerMetrics()
}

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

func (m *controllerMetrics) init(meter metric.Meter) {
	var err error

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
	if m.reconcileTotal == nil {
		return
	}

	m.reconcileTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("controller_name", controllerName)))
}

func (m *controllerMetrics) RecordRequeueIncrease(ctx context.Context, controllerName string, requeues int) {
	if m.requeueTotal == nil {
		return
	}

	m.requeueTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("controller_name", controllerName), attribute.Int("num_requeues", requeues)))
}

func (m *controllerMetrics) RecordQueueDrop(ctx context.Context, controllerName string) {
	if m.queueDropTotal == nil {
		return
	}

	m.queueDropTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("controller_name", controllerName)))
}
