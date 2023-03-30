package internal

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/version"
)

func ShowVersion(logger logr.Logger) {
	logger = logger.WithName("version")
	version.PrintVersionInfo(logger)
}
