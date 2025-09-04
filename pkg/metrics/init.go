package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	metricsConfig := NewMetricsConfigManager(logger, metricsConfiguration)

	SetManager(metricsConfig)

	if disableMetricsExport {
		err = metricsConfig.initializeMetrics(otel.GetMeterProvider())
		if err != nil {
			logger.Error(err, "failed initializing metrics")
			return nil, nil, nil, err
		}

		return metricsConfig, nil, nil, nil
	}

	var meterProvider metric.MeterProvider
	var metricsServerMux *http.ServeMux

	switch otelProvider {
	case "grpc":
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
	case "prometheus":
		meterProvider, err = NewPrometheusConfig(ctx, logger, metricsConfiguration)
		if err != nil {
			return nil, nil, nil, err
		}

		metricsServerMux = http.NewServeMux()
		metricsServerMux.Handle(config.MetricsPath, promhttp.Handler())
	}

	if meterProvider != nil {
		otel.SetMeterProvider(meterProvider)
	}

	err = metricsConfig.initializeMetrics(otel.GetMeterProvider())
	if err != nil {
		logger.Error(err, "failed initializing metrics")
		return nil, nil, nil, err
	}

	if otelProvider == "prometheus" && metricsConfiguration.GetMetricsRefreshInterval() > 0 {
		ticker := time.NewTicker(metricsConfiguration.GetMetricsRefreshInterval())
		go func() {
			for {
				select {
				case <-ticker.C:
					if p, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); ok {
						if err := p.Shutdown(ctx); err != nil {
							logger.Error(err, "failed to shutdown MeterProvider")
						}
					}

					meterProvider, err := NewPrometheusConfig(ctx, logger, metricsConfiguration)
					if err != nil {
						logger.Error(err, "failed to re-create MeterProvider")
						continue
					}

					otel.SetMeterProvider(meterProvider)

					err = metricsConfig.initializeMetrics(meterProvider)
					if err != nil {
						logger.Error(err, "failed re-initializing metrics")
						continue
					}

					logger.V(4).Info("restarted prometheus metrics")
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return metricsConfig, metricsServerMux, nil, nil
}
