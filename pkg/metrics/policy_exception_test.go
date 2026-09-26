package metrics

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestPolicyExceptionMetricObservesInventoryLabels(t *testing.T) {
	manager := NewMetricsConfigManager(logr.Discard(), config.NewDefaultMetricsConfiguration())
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	if err := manager.initializeMetrics(provider); err != nil {
		t.Fatalf("initializeMetrics() failed: %v", err)
	}
	exceptionMetrics := manager.PolicyExceptionMetrics()
	if _, err := exceptionMetrics.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		exceptionMetrics.RecordPolicyExceptionInfo(ctx, observer, "kyverno.io", "payments", "allow-old-app", "Policy", "payments/restrict-images")
		return nil
	}); err != nil {
		t.Fatalf("RegisterCallback() failed: %v", err)
	}

	var resourceMetrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &resourceMetrics); err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	var dataPoints []metricdata.DataPoint[int64]
	for _, scope := range resourceMetrics.ScopeMetrics {
		for _, metricData := range scope.Metrics {
			if metricData.Name != "kyverno_policy_exception_info" {
				continue
			}
			gauge, ok := metricData.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("metric data type = %T, want int64 gauge", metricData.Data)
			}
			dataPoints = gauge.DataPoints
		}
	}
	if len(dataPoints) != 1 {
		t.Fatalf("got %d data points, want 1", len(dataPoints))
	}
	point := dataPoints[0]
	if point.Value != 1 {
		t.Fatalf("value = %d, want 1", point.Value)
	}
	want := map[attribute.Key]string{
		"exception_api_group": "kyverno.io",
		"exception_namespace": "payments",
		"exception_name":      "allow-old-app",
		"policy_kind":         "Policy",
		"policy_name":         "payments/restrict-images",
	}
	for key, expected := range want {
		value, ok := point.Attributes.Value(key)
		if !ok || value.AsString() != expected {
			t.Errorf("attribute %q = %q, want %q", key, value.AsString(), expected)
		}
	}
}
