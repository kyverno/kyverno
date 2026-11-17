package internal

import (
	"github.com/go-logr/logr"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
)

// setupImageVerifyCache returns nil when the cache is disabled; callers treat a
// nil Client as "no cache".
func setupImageVerifyCache(logger logr.Logger) imageverifycache.Client {
	logger = logger.WithName("image-verify-cache").WithValues("enabled", imageVerifyCacheEnabled, "maxsize", imageVerifyCacheMaxSize, "ttl", imageVerifyCacheTTLDuration)
	logger.V(2).Info("setup image verify cache...")
	if !imageVerifyCacheEnabled {
		return nil
	}
	imageVerifyCache, err := imageverifycache.New(
		imageverifycache.WithLogger(logger),
		imageverifycache.WithMaxSize(imageVerifyCacheMaxSize),
		imageverifycache.WithTTLDuration(imageVerifyCacheTTLDuration),
	)
	checkError(logger, err, "failed to create image verify cache client")
	return imageVerifyCache
}
