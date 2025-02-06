package internal

import (
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/automaxprocs/maxprocs"
)

func setupMaxProcs(logger logr.Logger) func() {
	logger = logger.WithName("maxprocs")
	logger.V(2).Info("setup maxprocs...")
	undo, err := maxprocs.Set(
		maxprocs.Logger(
			func(format string, args ...interface{}) {
				logger.V(4).Info(fmt.Sprintf(format, args...))
			},
		),
	)
	if err != nil {
		logger.Error(err, "failed to configure maxprocs")
	}
	return undo
}
