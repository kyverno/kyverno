package internal

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func shutdown(logger logr.Logger, sdowns ...context.CancelFunc) context.CancelFunc {
	return func() {
		for i := range sdowns {
			logger.Info("shuting down...")
			defer sdowns[i]()
		}
	}
}

func Setup() (context.Context, logr.Logger, metrics.MetricsConfigManager, context.CancelFunc) {
	logger := SetupLogger()
	ShowVersion(logger)
	sdownMaxProcs := SetupMaxProcs(logger)
	SetupProfiling(logger)
	client := CreateKubernetesClient(logger)
	metricsManager, sdownMetrics := SetupMetrics(logger, client)
	ctx, sdownSignals := SetupSignals(logger)
	return ctx, logger, metricsManager, shutdown(logger.WithName("shutdown"), sdownMaxProcs, sdownMetrics, sdownSignals)
}
