package internal

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
)

var Context = context.Background()

func setupSignals(logger logr.Logger) (context.Context, context.CancelFunc) {
	logger = logger.WithName("signals")
	logger.Info("setup signals...")
	return signal.NotifyContext(Context, os.Interrupt, syscall.SIGTERM)
}
