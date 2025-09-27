package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeletingMetrics() DeletingMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.DeletingMetrics()
}

type DeletingMetrics interface {
	RecordDeletedObject(ctx context.Context, kind, namespace string, policy v1alpha1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation)
	RecordDeletingFailure(ctx context.Context, kind, namespace string, policy v1alpha1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation)
}

type deletingMetrics struct {
	deletedObjectsTotal   metric.Int64Counter
	deletingFailuresTotal metric.Int64Counter

	logger logr.Logger
}

func (m *deletingMetrics) init(meter metric.Meter) {
	var err error

	m.deletedObjectsTotal, err = meter.Int64Counter(
		"kyverno_deleting_controller_deletedobjects",
		metric.WithDescription("can be used to track number of deleted objects."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, deleting_controller_deletedobjects_total")
	}
	m.deletingFailuresTotal, err = meter.Int64Counter(
		"kyverno_deleting_controller_errors",
		metric.WithDescription("can be used to track number of deleting failures."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, deleting_controller_errors_total")
	}
}

func (m *deletingMetrics) RecordDeletedObject(ctx context.Context, kind, namespace string, policy v1alpha1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation) {
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

func (m *deletingMetrics) RecordDeletingFailure(ctx context.Context, kind, namespace string, policy v1alpha1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation) {
	if m.deletingFailuresTotal == nil {
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

	m.deletingFailuresTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}
