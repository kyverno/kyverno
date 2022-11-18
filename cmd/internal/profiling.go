package internal

import (
	"net"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/profiling"
)

func SetupProfiling(logger logr.Logger) {
	logger = logger.WithName("profiling").WithValues("enabled", profilingEnabled, "address", profilingAddress, "port", profilingPort)
	logger.Info("start profiling...")
	if profilingEnabled {
		profiling.Start(logger, net.JoinHostPort(profilingAddress, profilingPort))
	}
}
