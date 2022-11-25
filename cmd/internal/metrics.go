package internal

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/kubernetes"
)

func GetMetricsConfiguration(logger logr.Logger, client kubernetes.Interface) config.MetricsConfiguration {
	logger.Info("load metrics configuration...")
	metricsConfiguration, err := config.NewMetricsConfiguration(client)
	checkError(logger, err, "failed to load metrics configuration")
	return metricsConfiguration
}

func SetupMetrics(logger logr.Logger, kubeClient kubernetes.Interface) (metrics.MetricsConfigManager, context.CancelFunc) {
	logger = logger.WithName("metrics")
	logger.Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds)
	metricsConfiguration := GetMetricsConfiguration(logger, kubeClient)
	metricsAddr := ":" + metricsPort
	metricsConfig, metricsServerMux, metricsPusher, err := metrics.InitMetrics(
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
			if err := http.ListenAndServe(metricsAddr, metricsServerMux); err != nil {
				logger.Error(err, "failed to enable metrics", "address", metricsAddr)
			}
		}()
	}
	return metricsConfig, cancel
}
