package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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
	RecordDeletedObject(ctx context.Context, resource, namespace string, policy v1beta1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation)
	RecordDeletingFailure(ctx context.Context, resource, namespace string, policy v1beta1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation)
	// RecordDeletedResource emits a per-resource deletion counter including resource_name and resource_namespace labels.
	RecordDeletedResource(ctx context.Context, resourceKind, resourceNamespace, resourceName string, policy v1beta1.DeletingPolicyLike)
	// RecordDeletedResourceFailure emits a per-resource failure counter including resource_name and resource_namespace labels.
	RecordDeletedResourceFailure(ctx context.Context, resourceKind, resourceNamespace, resourceName string, policy v1beta1.DeletingPolicyLike)
}

type deletingMetrics struct {
	deletedObjectsTotal          metric.Int64Counter
	deletingFailuresTotal        metric.Int64Counter
	deletedResourcesTotal        metric.Int64Counter
	deletedResourceFailuresTotal metric.Int64Counter

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
	m.deletedResourcesTotal, err = meter.Int64Counter(
		"kyverno_deleting_controller_deleted_resources_total",
		metric.WithDescription("Per-resource counter of objects successfully deleted by a DeletingPolicy. Labels include resource_kind, resource_namespace, resource_name, policy_name, and policy_namespace."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, deleting_controller_deleted_resources_total")
	}
	m.deletedResourceFailuresTotal, err = meter.Int64Counter(
		"kyverno_deleting_controller_deleted_resource_failures_total",
		metric.WithDescription("Per-resource counter of deletion failures by a DeletingPolicy. Labels include resource_kind, resource_namespace, resource_name, policy_name, and policy_namespace."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, deleting_controller_deleted_resource_failures_total")
	}
}

func (m *deletingMetrics) RecordDeletedObject(ctx context.Context, resource, namespace string, policy v1beta1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation) {
	if m.deletedObjectsTotal == nil {
		return
	}

	labels := []attribute.KeyValue{
		attribute.String("policy_type", policy.GetKind()),
		attribute.String("policy_namespace", policy.GetNamespace()),
		attribute.String("policy_name", policy.GetName()),
		attribute.String("resource", resource),
	}

	if namespace != "" {
		labels = append(labels, attribute.String("resource_namespace", namespace))
	}

	if deletionPropagation != nil {
		labels = append(labels, attribute.String("deletion_propagation", string(*deletionPropagation)))
	}

	m.deletedObjectsTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}

func (m *deletingMetrics) RecordDeletingFailure(ctx context.Context, resource, namespace string, policy v1beta1.DeletingPolicyLike, deletionPropagation *metav1.DeletionPropagation) {
	if m.deletingFailuresTotal == nil {
		return
	}

	labels := []attribute.KeyValue{
		attribute.String("policy_type", policy.GetKind()),
		attribute.String("policy_namespace", policy.GetNamespace()),
		attribute.String("policy_name", policy.GetName()),
		attribute.String("resource", resource),
	}

	if namespace != "" {
		labels = append(labels, attribute.String("resource_namespace", namespace))
	}

	if deletionPropagation != nil {
		labels = append(labels, attribute.String("deletion_propagation", string(*deletionPropagation)))
	}

	m.deletingFailuresTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}

func (m *deletingMetrics) RecordDeletedResource(ctx context.Context, resourceKind, resourceNamespace, resourceName string, policy v1beta1.DeletingPolicyLike) {
	if m.deletedResourcesTotal == nil {
		return
	}
	labels := []attribute.KeyValue{
		attribute.String("policy_type", policy.GetKind()),
		attribute.String("policy_namespace", policy.GetNamespace()),
		attribute.String("policy_name", policy.GetName()),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_name", resourceName),
	}
	m.deletedResourcesTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}

func (m *deletingMetrics) RecordDeletedResourceFailure(ctx context.Context, resourceKind, resourceNamespace, resourceName string, policy v1beta1.DeletingPolicyLike) {
	if m.deletedResourceFailuresTotal == nil {
		return
	}
	labels := []attribute.KeyValue{
		attribute.String("policy_type", policy.GetKind()),
		attribute.String("policy_namespace", policy.GetNamespace()),
		attribute.String("policy_name", policy.GetName()),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_name", resourceName),
	}
	m.deletedResourceFailuresTotal.Add(ctx, 1, metric.WithAttributes(labels...))
}
