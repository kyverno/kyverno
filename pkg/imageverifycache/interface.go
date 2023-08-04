package imageverifycache

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

type Client interface {
	// Set Adds an image to the cache. The image is considered to be verified for the given rule in the policy
	// The entry outomatically expires after sometime
	// Returns true when the cache entry is added
	Set(ctx context.Context, policy kyvernov1.PolicyInterface, ruleName string, imageRef string) (bool, error)

	// Get Searches for the image verified using the rule in the policy in the cache
	// Returns true when the cache entry is found
	Get(ctx context.Context, policy kyvernov1.PolicyInterface, ruleName string, imagerRef string) (bool, error)
}
