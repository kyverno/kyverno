package internal

import (
	"net"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/profiling"
)

func setupProfiling(logger logr.Logger) {
	logger = logger.WithName("profiling").WithValues("enabled", profilingEnabled, "address", profilingAddress, "port", profilingPort)
	if profilingEnabled {
		logger.Info("setup profiling...")
		profiling.Start(logger, net.JoinHostPort(profilingAddress, profilingPort))
	}
}
