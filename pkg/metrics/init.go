package metrics

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel/sdk/metric"
	"k8s.io/client-go/kubernetes"
)

func InitMetrics(
	ctx context.Context,
	disableMetricsExport bool,
	otel string,
	metricsAddr string,
	otelCollector string,
	metricsConfiguration config.MetricsConfiguration,
	transportCreds string,
	kubeClient kubernetes.Interface,
	log logr.Logger,
) (MetricsConfigManager, *http.ServeMux, *metric.MeterProvider, error) {
	var err error
	var metricsServerMux *http.ServeMux

	metricsConfig := MetricsConfig{
		Log:    log,
		config: metricsConfiguration,
	}

	err = metricsConfig.initializeMetrics()
	if err != nil {
		log.Error(err, "Failed initializing metrics")
		return nil, nil, nil, err
	}

	if !disableMetricsExport {
		if otel == "grpc" {
			// Otlpgrpc metrics will be served on port 4317: default port for otlpgrpcmetrics
			log.V(2).Info("Enabling Metrics for Kyverno", "address", metricsAddr)

			endpoint := otelCollector + metricsAddr
			_, err := NewOTLPGRPCConfig(
				ctx,
				endpoint,
				transportCreds,
				kubeClient,
				log,
			)
			if err != nil {
				return nil, nil, nil, err
			}
		} else if otel == "prometheus" {
			// Prometheus Server will serve metrics on metrics-port
			metricsServerMux, err = NewPrometheusConfig(ctx, log)

			if err != nil {
				return nil, nil, nil, err
			}
		}
	}
	return &metricsConfig, metricsServerMux, nil, nil
}
