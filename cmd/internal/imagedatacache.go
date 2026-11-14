package internal

import (
	"github.com/go-logr/logr"
	imagedatacache "github.com/kyverno/kyverno/pkg/image/datacache"
)

// setupImageDataCache returns nil when the cache is disabled; callers treat a
// nil Client as "no cache".
func setupImageDataCache(logger logr.Logger) imagedatacache.Client {
	logger = logger.WithName("image-data-cache").WithValues("enabled", imageDataCacheEnabled, "maxsize", imageDataCacheMaxSize, "ttl", imageDataCacheTTLDuration, "tagttl", imageDataCacheTagTTLDuration)
	logger.V(2).Info("setup image data cache...")
	if !imageDataCacheEnabled {
		return nil
	}
	imageDataCache, err := imagedatacache.New(
		imagedatacache.WithMaxSize(imageDataCacheMaxSize),
		imagedatacache.WithTTLDuration(imageDataCacheTTLDuration),
		imagedatacache.WithTagTTLDuration(imageDataCacheTagTTLDuration),
	)
	checkError(logger, err, "failed to create image data cache client")
	return imageDataCache
}
