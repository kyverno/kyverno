package internal

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/version"
)

func showVersion(logger logr.Logger) {
	logger = logger.WithName("version")
	version.PrintVersionInfo(logger)
}
