package metrics

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = log.Log.WithName("metrics")

func Logger() logr.Logger {
	return logger
}
