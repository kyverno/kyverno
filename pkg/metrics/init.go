package metrics

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"k8s.io/client-go/kubernetes"
)

func InitMetrics(
	ctx context.Context,
	disableMetricsExport bool,
	otelProvider string,
	metricsAddr string,
	otelCollector string,
	metricsConfiguration config.MetricsConfiguration,
	transportCreds string,
	kubeClient kubernetes.Interface,
	logger logr.Logger,
) (MetricsConfigManager, *http.ServeMux, *sdkmetric.MeterProvider, error) {
	var err error
	var metricsServerMux *http.ServeMux
	if !disableMetricsExport {
		var meterProvider metric.MeterProvider
		if otelProvider == "grpc" {
			endpoint := otelCollector + metricsAddr
			meterProvider, err = NewOTLPGRPCConfig(
				ctx,
				endpoint,
				transportCreds,
				kubeClient,
				logger,
				metricsConfiguration,
			)
			if err != nil {
				return nil, nil, nil, err
			}
		} else if otelProvider == "prometheus" {
			meterProvider, metricsServerMux, err = NewPrometheusConfig(ctx, logger, metricsConfiguration)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		if meterProvider != nil {
			otel.SetMeterProvider(meterProvider)
		}
	}
	metricsConfig := MetricsConfig{
		Log:    logger,
		config: metricsConfiguration,
	}
	err = metricsConfig.initializeMetrics(otel.GetMeterProvider())
	if err != nil {
		logger.Error(err, "Failed initializing metrics")
		return nil, nil, nil, err
	}
	return &metricsConfig, metricsServerMux, nil, nil
}
