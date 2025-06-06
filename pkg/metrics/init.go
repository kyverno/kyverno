package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
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
) (MetricsConfigManager, *metric.MeterProvider, error) {
	var err error
	var meterProvider *metric.MeterProvider
	if !disableMetricsExport {
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
				return nil, nil, err
			}
		} else if otelProvider == "prometheus" {
			meterProvider, err = NewPrometheusConfig(ctx, logger, metricsConfiguration)
			if err != nil {
				return nil, nil, err
			}
		}
		if meterProvider != nil {
			otel.SetMeterProvider(meterProvider)
		}
	}
	metricsConfig := MetricsConfig{
		Log:            logger,
		metricsAddress: metricsAddr,
		otelProvider:   otelProvider,
		config:         metricsConfiguration,
	}
	err = metricsConfig.initializeMetrics(otel.GetMeterProvider())
	if err != nil {
		logger.Error(err, "Failed initializing metrics")
		return nil, nil, err
	}
	return &metricsConfig, meterProvider, nil
}
