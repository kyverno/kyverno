package internal

import (
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/automaxprocs/maxprocs"
)

func SetupMaxProcs(logger logr.Logger) func() {
	logger = logger.WithName("maxprocs")
	logger.Info("setup maxprocs...")
	undo, err := maxprocs.Set(
		maxprocs.Logger(
			func(format string, args ...interface{}) {
				logger.Info(fmt.Sprintf(format, args...))
			},
		),
	)
	checkError(logger, err, "failed to configure maxprocs")
	return undo
}
