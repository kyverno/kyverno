package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	otlp "go.opentelemetry.io/otel"
	"k8s.io/client-go/kubernetes"
)

func SetupMetrics(ctx context.Context, logger logr.Logger, metricsConfiguration config.MetricsConfiguration, kubeClient kubernetes.Interface) (metrics.MetricsConfigManager, context.CancelFunc) {
	logger = logger.WithName("metrics")
	logger.V(2).Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds, "tlsSecretName", metricsTlsSecretName)
	metricsAddr := fmt.Sprintf("[%s]:%d", metricsHost, metricsPort)
	// in case of otel collector being GRPC the metrics Host is the target address instead of the listening address
	metricsConfig, tlsProvider, metricsServerMux, metricsPusher, err := metrics.InitMetrics(
		ctx,
		disableMetricsExport,
		otel,
		metricsPort,
		otelCollector,
		metricsConfiguration,
		transportCreds,
		kubeClient,
		metricsCaSecretName,
		metricsTlsSecretName,
		logging.WithName("metrics"),
	)
	checkError(logger, err, "failed to init metrics")
	// Pass logger to opentelemetry so JSON format is used (when configured)
	otlp.SetLogger(logger)
	var cancel context.CancelFunc
	if otel == "grpc" {
		cancel = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			metrics.ShutDownController(ctx, metricsPusher)
		}
	}
	if otel == "prometheus" {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		if metricsTlsSecretName != "" {
			tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				certPem, keyPem, err := tlsProvider()
				if err != nil {
					return nil, err
				}
				pair, err := tls.X509KeyPair(certPem, keyPem)
				if err != nil {
					return nil, err
				}
				return &pair, nil
			}
			tlsConfig.CipherSuites = []uint16{
				// AEADs w/ ECDHE
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			}
		}
		go func() {
			server := &http.Server{
				Addr:              metricsAddr,
				TLSConfig:         tlsConfig,
				Handler:           metricsServerMux,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      30 * time.Second,
				ReadHeaderTimeout: 30 * time.Second,
				IdleTimeout:       5 * time.Minute,
				ErrorLog:          logging.StdLogger(logging.WithName("prometheus-server"), ""),
			}
			if metricsTlsSecretName != "" {
				logger.Info("Starting HTTPS metrics server", "address", metricsAddr)
				if err := server.ListenAndServeTLS("", ""); err != nil {
					logger.Error(err, "failed to enable TLS metrics server", "address", metricsAddr)
				}
			} else {
				logger.Info("Starting HTTP metrics server", "address", metricsAddr)
				if err := server.ListenAndServe(); err != nil {
					logger.Error(err, "failed to enable metrics server", "address", metricsAddr)
				}
			}
		}()
	}
	return metricsConfig, cancel
}
