package metrics

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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
	logger logr.Logger,
) (MetricsConfigManager, *http.ServeMux, *sdkmetric.MeterProvider, error) {
	var err error
	var metricsServerMux *http.ServeMux
	if !disableMetricsExport {
		var meterProvider metric.MeterProvider
		if otel == "grpc" {
			endpoint := otelCollector + metricsAddr
			meterProvider, err = NewOTLPGRPCConfig(
				ctx,
				endpoint,
				transportCreds,
				kubeClient,
				logger,
			)
			if err != nil {
				return nil, nil, nil, err
			}
		} else if otel == "prometheus" {
			meterProvider, metricsServerMux, err = NewPrometheusConfig(ctx, logger)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		if meterProvider != nil {
			global.SetMeterProvider(meterProvider)
		}
	}
	metricsConfig := MetricsConfig{
		Log:    logger,
		config: metricsConfiguration,
	}
	err = metricsConfig.initializeMetrics(global.MeterProvider())
	if err != nil {
		logger.Error(err, "Failed initializing metrics")
		return nil, nil, nil, err
	}
	return &metricsConfig, metricsServerMux, nil, nil
}
