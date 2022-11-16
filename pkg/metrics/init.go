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
	log logr.Logger,
) (*MetricsConfig, *http.ServeMux, *controller.Controller, error) {
	var err error
	var metricsServerMux *http.ServeMux
	var pusher *controller.Controller

	metricsConfig := new(MetricsConfig)
	metricsConfig.Log = log
	metricsConfig.Config = metricsConfigData

	metricsConfig, err = initializeMetrics(metricsConfig)
	if err != nil {
		log.Error(err, "Failed initializing metrics")
		return nil, nil, nil, err
	}

	if !disableMetricsExport {
		if otel == "grpc" {
			// Otlpgrpc metrics will be served on port 4317: default port for otlpgrpcmetrics
			log.V(2).Info("Enabling Metrics for Kyverno", "address", metricsAddr)

			endpoint := otelCollector + metricsAddr
			pusher, err = NewOTLPGRPCConfig(
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
			metricsServerMux, err = NewPrometheusConfig(log)

			if err != nil {
				return nil, nil, pusher, err
			}
		}
	}
	return metricsConfig, metricsServerMux, pusher, nil
}
