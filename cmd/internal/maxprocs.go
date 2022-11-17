package internal

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"go.uber.org/automaxprocs/maxprocs"
)

func SetupMaxProcs(logger logr.Logger) func() {
	logger = logger.WithName("maxprocs")
	undo, err := maxprocs.Set(
		maxprocs.Logger(
			func(format string, args ...interface{}) {
				logger.Info(fmt.Sprintf(format, args...))
			},
		),
	)
	if err != nil {
		logger.Error(err, "failed to configure maxprocs")
		os.Exit(1)
	}
	return undo
}
