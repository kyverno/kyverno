package internal

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	"github.com/kyverno/kyverno/pkg/informers"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	kyvernotls "github.com/kyverno/kyverno/pkg/tls"
	otlp "go.opentelemetry.io/otel"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

func SetupMetrics(ctx context.Context, logger logr.Logger, metricsConfiguration config.MetricsConfiguration, kubeClient kubernetes.Interface) (metrics.MetricsConfigManager, context.CancelFunc) {
	logger = logger.WithName("metrics")
	logger.V(2).Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds, "tlsSecretName", metricsTlsSecretName)
	metricsAddr := fmt.Sprintf("[%s]:%d", metricsHost, metricsPort)

	var tlsSecretInformer corev1informers.SecretInformer
	var caSecretInformer corev1informers.SecretInformer
	if metricsTlsSecretName != "" {
		logger.Info("Metrics TLS secret name is provided, metrics server will use TLS")
		tlsSecretInformer = informers.NewSecretInformer(kubeClient, config.KyvernoNamespace(), metricsTlsSecretName, resyncPeriod)
		caSecretInformer = informers.NewSecretInformer(kubeClient, config.KyvernoNamespace(), metricsCaSecretName, resyncPeriod)
		if !informers.StartInformersAndWaitForCacheSync(ctx, logger, caSecretInformer, tlsSecretInformer) {
			logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
	}
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
		tlsSecretInformer,
		caSecretInformer,
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
	// Setup certificate renewer for metrics server.
	// Only setup if metricsTlsSecretName is provided.
	if metricsTlsSecretName != "" {
		metricsKeyAlgorithm, ok := kyvernotls.KeyAlgorithms[strings.ToUpper(metricsKeyAlgorithm)]
		if !ok {
			logger.Error(fmt.Errorf("unsupported key algorithm: %s (supported: RSA, ECDSA, Ed25519)", metricsKeyAlgorithm), "invalid tlsKeyAlgorithm flag")
			os.Exit(1)
		}

		renewer := kyvernotls.NewCertRenewer(
			kubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
			kyvernotls.CertRenewalInterval,
			kyvernotls.CAValidityDuration,
			kyvernotls.TLSValidityDuration,
			renewBefore,
			serverIP,
			config.KyvernoServiceName(),
			config.DnsNames(config.KyvernoServiceName(), config.KyvernoNamespace()),
			config.KyvernoNamespace(),
			metricsCaSecretName,
			metricsTlsSecretName,
			metricsKeyAlgorithm,
		)
		certController := NewController(
			certmanager.ControllerName,
			certmanager.NewController(
				caSecretInformer,
				tlsSecretInformer,
				renewer,
				metricsCaSecretName,
				metricsTlsSecretName,
				config.KyvernoNamespace(),
			),
			certmanager.Workers,
		)

		var wg wait.Group
		certController.Run(ctx, logger, &wg)
	}

	return metricsConfig, cancel
}
