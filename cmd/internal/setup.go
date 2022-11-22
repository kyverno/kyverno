package internal

import (
	"context"

	"github.com/go-logr/logr"
)

func Setup() (context.Context, logr.Logger, context.CancelFunc) {
	logger := SetupLogger()
	ShowVersion(logger)
	sdownMaxProcs := SetupMaxProcs(logger)
	SetupProfiling(logger)
	ctx, sdownSignals := SetupSignals(logger)
	return ctx, logger, func() {
		sdownSignals()
		sdownMaxProcs()
	}
}
