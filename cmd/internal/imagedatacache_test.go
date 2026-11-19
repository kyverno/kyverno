package internal

import (
	"context"
	"flag"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	imagedatacache "github.com/kyverno/kyverno/pkg/image/datacache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// these tests change package level flag variables, so they must not run in parallel.
func TestSetupImageDataCache(t *testing.T) {
	prevEnabled, prevSize, prevTTL, prevTagTTL := imageDataCacheEnabled, imageDataCacheMaxSize, imageDataCacheTTLDuration, imageDataCacheTagTTLDuration
	t.Cleanup(func() {
		imageDataCacheEnabled, imageDataCacheMaxSize, imageDataCacheTTLDuration, imageDataCacheTagTTLDuration = prevEnabled, prevSize, prevTTL, prevTagTTL
	})

	imageDataCacheEnabled = false
	assert.Nil(t, setupImageDataCache(logr.Discard()), "a disabled cache must be a nil Client, which callers treat as no cache")

	// different TTLs, so a swapped or dropped option shows up as the wrong reference kind being cached
	imageDataCacheEnabled = true
	imageDataCacheMaxSize = imagedatacache.DefaultMaxSize
	imageDataCacheTTLDuration = 0
	imageDataCacheTagTTLDuration = time.Minute
	c := setupImageDataCache(logr.Discard())
	require.NotNil(t, c)
	fetches := func(image string) int {
		calls := 0
		fetch := func(context.Context) (map[string]any, error) { calls++; return map[string]any{"digest": "d"}, nil }
		for range 2 {
			_, err := c.Get(t.Context(), image, fetch)
			require.NoError(t, err)
		}
		return calls
	}
	assert.Equal(t, 1, fetches("nginx:1.25"), "tag references use the tag TTL")
	assert.Equal(t, 2, fetches("nginx@sha256:"+strings.Repeat("a", 64)), "digest references use the digest TTL, which is disabled here")
}

// initFlags re-registers every flag, which resets the bound variables to their defaults and sets the klog verbosity.
func TestImageDataCacheFlagsAreOptIn(t *testing.T) {
	previous := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = previous })
	names := []string{"imageDataCacheEnabled", "imageDataCacheMaxSize", "imageDataCacheTTLDuration", "imageDataCacheTagTTLDuration"}

	flag.CommandLine = flag.NewFlagSet("without", flag.ContinueOnError)
	initFlags(NewConfiguration(WithKubeconfig()))
	for _, n := range names {
		assert.Nil(t, flag.Lookup(n), "%s must only exist for binaries that use the image data cache", n)
	}

	flag.CommandLine = flag.NewFlagSet("with", flag.ContinueOnError)
	initFlags(NewConfiguration(WithImageDataCache()))
	for _, n := range names {
		assert.NotNil(t, flag.Lookup(n), n)
	}
	assert.Equal(t, "false", flag.Lookup("imageDataCacheEnabled").DefValue, "the cache is opt-in")
	assert.Equal(t, strconv.FormatInt(imagedatacache.DefaultMaxSize, 10), flag.Lookup("imageDataCacheMaxSize").DefValue)
	assert.Equal(t, imagedatacache.DefaultTTL.String(), flag.Lookup("imageDataCacheTTLDuration").DefValue)
	assert.Equal(t, imagedatacache.DefaultTagTTL.String(), flag.Lookup("imageDataCacheTagTTLDuration").DefValue)
}
