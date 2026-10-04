package metrics

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	corev1 "k8s.io/api/core/v1"
)

func TestPolicyExceptionPrometheusLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := config.NewDefaultMetricsConfiguration()
	manager := NewMetricsConfigManager(logr.Discard(), cfg)
	m := manager.PolicyExceptionMetrics()
	// Register before initialization as well as retain the callback on refresh.
	references := []string{"team/old", "team/other"}
	_, err := m.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		for _, policy := range references {
			m.RecordPolicyExceptionInfo(ctx, observer, "kyverno.io", "team", "exception", "Policy", policy)
		}
		return nil
	})
	require.NoError(t, err)
	var provider *sdkmetric.MeterProvider
	var registry *prometheus.Registry
	refresh := func() {
		t.Helper()
		if provider != nil {
			require.NoError(t, provider.Shutdown(ctx))
		}
		registry = prometheus.NewRegistry()
		exporter, err := otelprometheus.New(
			otelprometheus.WithRegisterer(registry),
			otelprometheus.WithoutUnits(),
			otelprometheus.WithoutTargetInfo(),
		)
		require.NoError(t, err)
		provider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter), sdkmetric.WithView(cfg.BuildMeterProviderViews()...))
		require.NoError(t, manager.initializeMetrics(provider))
	}
	defer func() {
		if provider != nil {
			require.NoError(t, provider.Shutdown(ctx))
		}
	}()
	collect := func(want ...string) {
		t.Helper()
		families, err := registry.Gather()
		require.NoError(t, err)
		var names []string
		for _, family := range families {
			if family.GetName() != "kyverno_policy_exception_info" {
				continue
			}
			require.Equal(t, "GAUGE", family.GetType().String())
			for _, sample := range family.Metric {
				require.Equal(t, float64(1), sample.GetGauge().GetValue())
				got := map[string]string{}
				for _, label := range sample.Label {
					// OTel adds instrumentation scope labels, separate from
					// the five labels defining exception identity.
					if label.GetName() == "otel_scope_name" || label.GetName() == "otel_scope_version" || label.GetName() == "otel_scope_schema_url" {
						continue
					}
					got[label.GetName()] = label.GetValue()
				}
				require.Equal(t, map[string]string{
					"exception_api_group": "kyverno.io", "exception_namespace": "team",
					"exception_name": "exception", "policy_kind": "Policy",
					"policy_name": got["policy_name"],
				}, got)
				names = append(names, got["policy_name"])
			}
		}
		require.ElementsMatch(t, want, names)
	}
	refresh()
	collect("team/old", "team/other")
	response := httptest.NewRecorder()
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), "# TYPE kyverno_policy_exception_info gauge")

	references = []string{"team/new"}
	collect("team/new")
	refresh()
	collect("team/new")
	cfg.Load(&corev1.ConfigMap{Data: map[string]string{"namespaces": `{"include":["elsewhere"]}`}})
	collect()
	cfg.Load(&corev1.ConfigMap{Data: map[string]string{"namespaces": `{"include":["team"],"exclude":["team"]}`}})
	collect()
	cfg.Load(nil)
	collect("team/new")
	references = nil
	collect()
	references = []string{"team/recreated"}
	collect("team/recreated")
	cfg.Load(&corev1.ConfigMap{Data: map[string]string{"metricsExposure": `{"kyverno_policy_exception_info":{"enabled":false}}`}})
	refresh()
	collect()
}

func TestPolicyExceptionNoopMetrics(t *testing.T) {
	t.Parallel()
	m := NewMetricsConfigManager(logr.Discard(), config.NewDefaultMetricsConfiguration())
	require.NoError(t, m.initializeMetrics(noop.NewMeterProvider()))
	_, err := m.PolicyExceptionMetrics().RegisterCallback(func(context.Context, metric.Observer) error {
		t.Fatal("disabled metrics must not collect")
		return nil
	})
	require.NoError(t, err)
}
