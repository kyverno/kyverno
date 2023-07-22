package ttlcontroller

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/logging"
)

func CreateLogger(name string) logr.Logger {
	var logger = logging.WithName(name)
	return logger
}