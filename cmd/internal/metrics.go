package internal

import (
	"context"
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
	logger.V(2).Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds)
	metricsAddr := ":" + metricsPort
	metricsConfig, metricsProvider, err := metrics.InitMetrics(
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
	if otel == "grpc" || otel == "prometheus" {
		cancel = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			metrics.ShutDownController(ctx, metricsProvider)
		}
	}
	return metricsConfig, cancel
}
