package internal

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	extcertmanager "github.com/kyverno/pkg/certmanager"
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
	logger.V(2).Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds, "tlsSecretName", metricsTLSSecretName)
	metricsAddr := fmt.Sprintf("[%s]:%d", metricsHost, metricsPort)

	var (
		metricsTLSSecretInformer corev1informers.SecretInformer
		metricsCASecretInformer  corev1informers.SecretInformer
		keyAlgorithm             kyvernotls.KeyAlgorithm
		ok                       bool
	)

	if metricsTLSSecretName != "" {
		logger.Info("Metrics TLS secret name is provided, metrics server will use TLS")
		metricsTLSSecretInformer = informers.NewSecretInformer(kubeClient, config.KyvernoNamespace(), metricsTLSSecretName, resyncPeriod)
		metricsCASecretInformer = informers.NewSecretInformer(kubeClient, config.KyvernoNamespace(), metricsCASecretName, resyncPeriod)
		if !informers.StartInformersAndWaitForCacheSync(ctx, logger, metricsCASecretInformer, metricsTLSSecretInformer) {
			checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		}
		keyAlgorithm, ok = kyvernotls.KeyAlgorithms[strings.ToUpper(metricsKeyAlgorithm)]
		if !ok {
			checkError(logger, fmt.Errorf("unsupported key algorithm: %s (supported: RSA, ECDSA, Ed25519)", metricsKeyAlgorithm), "invalid tlsKeyAlgorithm flag")
		}
		// Create certificate renewer for metrics TLS.
		renewer := kyvernotls.NewCertRenewer(
			kubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
			kyvernotls.CertRenewalInterval,
			kyvernotls.CAValidityDuration,
			kyvernotls.TLSValidityDuration,
			metricsRenewBefore,
			metricsServerIP,
			config.KyvernoServiceName(),
			config.DnsNames(config.KyvernoServiceName(), config.KyvernoNamespace()),
			config.KyvernoNamespace(),
			metricsCASecretName,
			metricsTLSSecretName,
			keyAlgorithm,
		)
		certController := NewController(
			extcertmanager.ControllerName,
			NewCertManagerController(
				createClientConfig(logger, clientRateLimitQPS, clientRateLimitBurst),
				renewer,
				metricsCASecretName,
				metricsTLSSecretName,
				config.KyvernoNamespace(),
			),
			extcertmanager.Workers,
		)
		var wg wait.Group
		certController.Run(ctx, logger, &wg)
		// Wait for the certificate controller to create the TLS secrets
		// This ensures they exist before InitMetrics tries to use them
		if err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			caSecret, _ := metricsCASecretInformer.Lister().Secrets(config.KyvernoNamespace()).Get(metricsCASecretName)
			tlsSecret, _ := metricsTLSSecretInformer.Lister().Secrets(config.KyvernoNamespace()).Get(metricsTLSSecretName)
			return caSecret != nil && tlsSecret != nil, nil
		}); err != nil {
			checkError(logger, err, "timeout waiting for metrics TLS secrets to be created")
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
		metricsTLSSecretInformer,
		metricsCASecretInformer,
		metricsCASecretName,
		metricsTLSSecretName,
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
		if metricsTLSSecretName != "" {
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
			if metricsTLSSecretName != "" {
				logger.Info("Starting HTTPS metrics server", "address", metricsAddr)
				if err := server.ListenAndServeTLS("", ""); err != nil {
					checkError(logger, err, "failed to enable TLS encrypted metrics server", "address", metricsAddr)
				}
			} else {
				logger.Info("Starting HTTP metrics server", "address", metricsAddr)
				if err := server.ListenAndServe(); err != nil {
					checkError(logger, err, "failed to enable metrics server", "address", metricsAddr)
				}
			}
		}()
	}

	return metricsConfig, cancel
}
