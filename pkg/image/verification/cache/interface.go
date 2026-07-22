package cache

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client interface {
	// Set adds a verified image digest to the cache for the given policy rule.
	// The entry automatically expires after some time.
	// Returns true when the cache entry is added.
	Set(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, verifiedDigest string, useCache bool) (bool, error)

	// Get searches for a verified image digest in the cache for the given policy rule.
	// Returns whether an entry was found and the verified digest bound to that entry.
	Get(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, string, error)
}
