package internal

import (
	"context"

	"github.com/go-logr/logr"
)

func shutdown(logger logr.Logger, sdowns ...context.CancelFunc) context.CancelFunc {
	return func() {
		for i := range sdowns {
			logger.Info("shuting down...")
			defer sdowns[i]()
		}
	}
}

func Setup() (context.Context, logr.Logger, context.CancelFunc) {
	logger := SetupLogger()
	ShowVersion(logger)
	sdownMaxProcs := SetupMaxProcs(logger)
	SetupProfiling(logger)
	ctx, sdownSignals := SetupSignals(logger)
	return ctx, logger, shutdown(logger.WithName("shutdown"), sdownMaxProcs, sdownSignals)
}
