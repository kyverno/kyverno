package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func setupSignals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func main() {
	// setup signals
	signalCtx, signalCancel := setupSignals()
	defer signalCancel()
	// wait for termination signal
	<-signalCtx.Done()
}
