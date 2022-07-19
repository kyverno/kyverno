package metrics

import (
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"k8s.io/client-go/kubernetes"
)

func InitMetrics(
	disableMetricsExport bool,
	otel string,
	metricsAddr string,
	otelCollector string,
	metricsConfigData *config.MetricsConfigData,
	transportCreds string,
	kubeClient kubernetes.Interface,
	log logr.Logger) (*MetricsConfig, *http.ServeMux, *controller.Controller, error) {

	var metricsConfig *MetricsConfig
	var err error
	var metricsServerMux *http.ServeMux
	var pusher *controller.Controller
	if !disableMetricsExport {
		if otel == "grpc" {
			// Otlpgrpc metrics will be served on port 4317: default port for otlpgrpcmetrics
			log.Info("Enabling Metrics for Kyverno", "address", metricsAddr)

			endpoint := otelCollector + metricsAddr
			metricsConfig, pusher, err = NewOTLPGRPCConfig(
				endpoint,
				metricsConfigData,
				transportCreds,
				kubeClient,
				log,
			)
			if err != nil {
				return nil, nil, pusher, err
			}
		} else if otel == "prometheus" {
			// Prometheus Server will serve metrics on metrics-port
			metricsConfig, metricsServerMux, err = NewPrometheusConfig(metricsConfigData, log)

			if err != nil {
				return nil, nil, pusher, err
			}
		}
	}
	return metricsConfig, metricsServerMux, pusher, nil
}
