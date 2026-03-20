package cache

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client interface {
	// Set Adds an image to the cache. The image is considered to be verified for the given rule in the policy
	// The entry outomatically expires after sometime
	// Returns true when the cache entry is added
	Set(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, error)

	// Get Searches for the image verified using the rule in the policy in the cache
	// Returns true when the cache entry is found
	Get(ctx context.Context, policy metav1.Object, ruleName string, imagerRef string, useCache bool) (bool, error)
}
