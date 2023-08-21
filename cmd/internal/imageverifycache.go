package internal

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
)

func setupImageVerifyCache(ctx context.Context, logger logr.Logger) imageverifycache.Client {
	logger = logger.WithName("image-verify-cache").WithValues("enabled", imageVerifyCacheEnabled, "maxsize", imageVerifyCacheMaxSize, "ttl", imageVerifyCacheTTLDuration)
	logger.Info("setup image verify cache...")
	opts := []imageverifycache.Option{
		imageverifycache.WithLogger(logger),
		imageverifycache.WithCacheEnableFlag(imageVerifyCacheEnabled),
		imageverifycache.WithMaxSize(imageVerifyCacheMaxSize),
		imageverifycache.WithTTLDuration(imageVerifyCacheTTLDuration),
	}
	imageVerifyCache, err := imageverifycache.New(opts...)
	checkError(logger, err, "failed to create image verify cache client")
	return imageVerifyCache
}
