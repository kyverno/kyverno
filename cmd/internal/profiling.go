package internal

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/profiling"
)

func StartProfiling(logger logr.Logger) {
	logger = logger.WithName("profiling").WithValues("enabled", profilingEnabled, "address", profilingAddress, "port", profilingPort)
	logger.Info("start profiling...")
	if profilingEnabled {
		profiling.Start(logger, fmt.Sprintf("%s:%d", profilingAddress, profilingPort))
	}
}
