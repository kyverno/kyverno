package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetAdmissionMetrics() AdmissionMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.AdmissionMetrics()
}

type admissionMetrics struct {
	requestsMetric metric.Int64Counter
	durationMetric metric.Float64Histogram

	logger logr.Logger
}

type AdmissionMetrics interface {
	RecordRequest(ctx context.Context, allowed bool, namespace string, operation admissionv1.Operation, gvk schema.GroupVersionKind, startTime time.Time, attrs ...attribute.KeyValue)
}

func (m *admissionMetrics) init(meter metric.Meter) {
	var err error

	m.requestsMetric, err = meter.Int64Counter(
		"kyverno_admission_requests",
		metric.WithDescription("can be used to track the number of admission requests encountered by Kyverno in the cluster"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_admission_requests_total")
	}
	m.durationMetric, err = meter.Float64Histogram(
		"kyverno_admission_review_duration_seconds",
		metric.WithDescription("can be used to track the latencies (in seconds) associated with the entire individual admission review. For example, if an incoming request trigger, say, five policies, this metric will track the e2e latency associated with the execution of all those policies"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_admission_review_duration_seconds")
	}
}

func (m *admissionMetrics) RecordRequest(ctx context.Context, allowed bool, namespace string, operation admissionv1.Operation, gvk schema.GroupVersionKind, startTime time.Time, attrs ...attribute.KeyValue) {
	if m.durationMetric == nil || m.requestsMetric == nil {
		return
	}

	if !GetManager().Config().CheckNamespace(namespace) {
		return
	}

	attributes := []attribute.KeyValue{
		attribute.String("resource_kind", gvk.Kind),
		attribute.String("resource_namespace", namespace),
		attribute.String("resource_request_operation", strings.ToLower(string(operation))),
		attribute.Bool("request_allowed", allowed),
	}

	attributes = append(attributes, attrs...)

	defer func() {
		latency := int64(time.Since(startTime))
		durationInSeconds := float64(latency) / float64(1000*1000*1000)
		m.durationMetric.Record(ctx, durationInSeconds, metric.WithAttributes(attributes...))
	}()

	m.requestsMetric.Add(ctx, 1, metric.WithAttributes(attributes...))
}
