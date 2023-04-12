package internal

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/cosign"
)

func setupCosign(logger logr.Logger) {
	logger = logger.WithName("cosign").WithValues("repository", imageSignatureRepository)
	logger.Info("setup cosign...")
	if imageSignatureRepository != "" {
		cosign.ImageSignatureRepository = imageSignatureRepository
	}
}
