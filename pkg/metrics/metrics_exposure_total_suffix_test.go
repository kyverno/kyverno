package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	corev1 "k8s.io/api/core/v1"
)

// collectedMetricNames returns the set of instrument names present across all
// scopes in rm.
func collectedMetricNames(rm metricdata.ResourceMetrics) map[string]bool {
	names := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names[m.Name] = true
		}
	}
	return names
}

// TestMetricsExposure_DisableAppliesToCounterMetrics is a regression test for
// https://github.com/kyverno/kyverno/issues/9112.
//
// A user disables a counter through the metricsExposure configmap key using the
// metric's Prometheus-exported name, e.g. "kyverno_http_requests_total" - that is
// the only name Prometheus (and the OpenTelemetry-to-Prometheus bridge, which always
// appends "_total" to a counter's base name) ever shows them. BuildMeterProviderViews
// must still recognize the counter and drop it, exactly like it already does for
// histograms and gauges, whose names are not rewritten during export.
func TestMetricsExposure_DisableAppliesToCounterMetrics(t *testing.T) {
	cfg := kconfig.NewDefaultMetricsConfiguration()
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"metricsExposure": `{"kyverno_http_requests_total": {"enabled": false}}`,
		},
	})

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(cfg.BuildMeterProviderViews()...),
	)

	mcm := NewMetricsConfigManager(logr.Discard(), cfg)
	if err := mcm.initializeMetrics(provider); err != nil {
		t.Fatalf("initializeMetrics failed: %v", err)
	}

	mcm.HTTPMetrics().RecordRequest(context.Background(), "GET", "/healthz", time.Now())

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if names := collectedMetricNames(rm); names["kyverno_http_requests"] {
		t.Errorf("kyverno_http_requests was exported even though metricsExposure "+
			"disables kyverno_http_requests_total; exported metric names: %v", names)
	}
}
