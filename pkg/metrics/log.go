package metrics

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/logging"
)

var logger = logging.WithName("metrics")

func Logger() logr.Logger {
	return logger
}
