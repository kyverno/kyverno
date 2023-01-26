package internal

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func shutdown(logger logr.Logger, sdowns ...context.CancelFunc) context.CancelFunc {
	return func() {
		for i := range sdowns {
			if sdowns[i] != nil {
				logger.Info("shutting down...")
				defer sdowns[i]()
			}
		}
	}
}

func Setup(name string) (context.Context, logr.Logger, metrics.MetricsConfigManager, context.CancelFunc) {
	logger := SetupLogger()
	ShowVersion(logger)
	sdownMaxProcs := SetupMaxProcs(logger)
	SetupProfiling(logger)
	client := CreateKubernetesClient(logger)
	ctx, sdownSignals := SetupSignals(logger)
	metricsManager, sdownMetrics := SetupMetrics(ctx, logger, client)
	sdownTracing := SetupTracing(logger, name, client)
	return ctx, logger, metricsManager, shutdown(logger.WithName("shutdown"), sdownMaxProcs, sdownMetrics, sdownTracing, sdownSignals)
}
