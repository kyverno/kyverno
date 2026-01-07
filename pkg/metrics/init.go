package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	corev1 "k8s.io/api/core/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

type TlsProvider func() ([]byte, []byte, error)

func InitMetrics(
	ctx context.Context,
	disableMetricsExport bool,
	otelProvider string,
	metricsPort int,
	otelCollector string,
	metricsConfiguration config.MetricsConfiguration,
	transportCreds string,
	kubeClient kubernetes.Interface,
	tlsSecretInformer corev1informers.SecretInformer,
	caSecretInformer corev1informers.SecretInformer,
	metricsCaSecretName string,
	metricsTlsSecretName string,
	logger logr.Logger,
) (MetricsConfigManager, TlsProvider, *http.ServeMux, *sdkmetric.MeterProvider, error) {
	var err error

	metricsConfig := NewMetricsConfigManager(logger, metricsConfiguration)

	// Create TLS provider function that loads certificates from Kubernetes secrets
	tlsProvider := func() ([]byte, []byte, error) {
		if metricsTlsSecretName == "" {
			return nil, nil, nil
		}

		secret, err := tlsSecretInformer.Lister().Secrets(config.KyvernoNamespace()).Get(metricsTlsSecretName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get metrics TLS secret %s: %w", metricsTlsSecretName, err)
		}

		certPem, exists := secret.Data[corev1.TLSCertKey]
		if !exists {
			return nil, nil, fmt.Errorf("Metrics TLS certificate \"tls.crt\" not found in secret %s", metricsTlsSecretName)
		}

		keyPem, exists := secret.Data[corev1.TLSPrivateKeyKey]
		if !exists {
			return nil, nil, fmt.Errorf("Metrics TLS private key \"tls.key\" not found in secret %s", metricsTlsSecretName)
		}

		return certPem, keyPem, nil
	}
	_, _, err = tlsProvider()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create TLS provider for metrics: %w", err)
	}

	SetManager(metricsConfig)

	if disableMetricsExport {
		err = metricsConfig.initializeMetrics(otel.GetMeterProvider())
		if err != nil {
			logger.Error(err, "failed initializing metrics")
			return nil, nil, nil, nil, err
		}

		return metricsConfig, nil, nil, nil, nil
	}

	var meterProvider metric.MeterProvider
	var metricsServerMux *http.ServeMux

	switch otelProvider {
	case "grpc":
		endpoint := fmt.Sprintf("[%s]:%d", otelCollector, metricsPort)
		meterProvider, err = NewOTLPGRPCConfig(
			ctx,
			endpoint,
			transportCreds,
			kubeClient,
			logger,
			metricsConfiguration,
		)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	case "prometheus":
		meterProvider, err = NewPrometheusConfig(ctx, logger, metricsConfiguration)
		if err != nil {
			return nil, nil, nil, nil, err
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
		return nil, nil, nil, nil, err
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

	return metricsConfig, tlsProvider, metricsServerMux, nil, nil
}
