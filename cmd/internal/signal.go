package internal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

var Context = context.Background()

func SetupSignals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(Context, os.Interrupt, syscall.SIGTERM)
}
