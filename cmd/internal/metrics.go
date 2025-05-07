package internal

import (
	"context"
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
	logger.Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds)
	metricsAddr := ":" + metricsPort
	metricsConfig, metricsServerMux, metricsPusher, err := metrics.InitMetrics(
		ctx,
		disableMetricsExport,
		otel,
		metricsAddr,
		otelCollector,
		metricsConfiguration,
		transportCreds,
		kubeClient,
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
		go func() {
			server := &http.Server{
				Addr:              metricsAddr,
				Handler:           metricsServerMux,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      30 * time.Second,
				ReadHeaderTimeout: 30 * time.Second,
				IdleTimeout:       5 * time.Minute,
				ErrorLog:          logging.StdLogger(logging.WithName("prometheus-server"), ""),
			}
			if err := server.ListenAndServe(); err != nil {
				logger.Error(err, "failed to enable metrics", "address", metricsAddr)
			}
		}()
	}
	return metricsConfig, cancel
}
