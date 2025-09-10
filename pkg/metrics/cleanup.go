package metrics

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCleanupMetrics() CleanupMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.CleanupMetrics()
}

type CleanupMetrics interface {
	RecordDeletedObject(ctx context.Context, kind, namespace string, policy kyvernov2.CleanupPolicyInterface, deletionPropagation *metav1.DeletionPropagation)
	RecordCleanupFailure(ctx context.Context, kind, namespace string, policy kyvernov2.CleanupPolicyInterface, deletionPropagation *metav1.DeletionPropagation)
}

type cleanupMetrics struct {
	deletedObjectsTotal  metric.Int64Counter
	cleanupFailuresTotal metric.Int64Counter

	logger logr.Logger
}

func (m *cleanupMetrics) init(meter metric.Meter) {
	var err error

	m.deletedObjectsTotal, err = meter.Int64Counter(
		"kyverno_cleanup_controller_deletedobjects",
		metric.WithDescription("can be used to track number of deleted objects."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, cleanup_controller_deletedobjects_total")
	}
	m.cleanupFailuresTotal, err = meter.Int64Counter(
		"kyverno_cleanup_controller_errors",
		metric.WithDescription("can be used to track number of cleanup failures."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, cleanup_controller_errors_total")
	}
}

func (m *cleanupMetrics) RecordDeletedObject(ctx context.Context, kind, namespace string, policy kyvernov2.CleanupPolicyInterface, deletionPropagation *metav1.DeletionPropagation) {
	if m.deletedObjectsTotal == nil {
		return
	}

	labels := []attribute.KeyValue{
		attribute.String("policy_type", policy.GetKind()),
		attribute.String("policy_namespace", policy.GetNamespace()),
		attribute.String("policy_name", policy.GetName()),
		attribute.String("resource_kind", kind),
	}

	if namespace != "" {
		labels = append(labels, attribute.String("resource_namespace", namespace))
	}

	if deletionPropagation != nil {
		labels = append(labels, attribute.String("deletion_propagation", string(*deletionPropagation)))
	}

	m.deletedObjectsTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}

func (m *cleanupMetrics) RecordCleanupFailure(ctx context.Context, kind, namespace string, policy kyvernov2.CleanupPolicyInterface, deletionPropagation *metav1.DeletionPropagation) {
	if m.cleanupFailuresTotal == nil {
		return
	}

	labels := []attribute.KeyValue{
		attribute.String("policy_type", policy.GetKind()),
		attribute.String("policy_namespace", policy.GetNamespace()),
		attribute.String("policy_name", policy.GetName()),
		attribute.String("resource_kind", kind),
	}

	if namespace != "" {
		labels = append(labels, attribute.String("resource_namespace", namespace))
	}

	if deletionPropagation != nil {
		labels = append(labels, attribute.String("deletion_propagation", string(*deletionPropagation)))
	}

	m.cleanupFailuresTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}
