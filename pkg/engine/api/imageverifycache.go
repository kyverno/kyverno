package api

import "context"

type ImageVerifyCacheClient interface {
	// Set Adds an image to the cache. The image is considered to be verified for the given rule in the policy
	// The entry outomatically expires after sometime
	// Returns true when the cache entry is added
	Set(ctx context.Context, policyId string, ruleName string, imageRef string) (bool, error)

	// Get Searches for the image verified using the rule in the policy in the cache
	// Returns true when the cache entry is found
	Get(ctx context.Context, policyId string, ruleName string, imagerRef string) (bool, error)

	// Delete deletes a specific image entry that was verified using the given rule
	// Returns true when the cache entry is successfully deleted
	Delete(ctx context.Context, policyId string, ruleName string, imageRef string) (bool, error)

	// Delete for rule delete all entries for a given rule
	// Returns true when all entries are successfully deleted
	DeleteForRule(ctx context.Context, policyId string, ruleName string) (bool, error)

	// Delete for rule delete all entries for a given policy
	// Returns true when all entries are successfully deleted
	DeleteForPolicy(ctx context.Context, policyId string) (bool, error)

	// Clear clears the entire cache
	Clear(ctx context.Context) (bool, error)
}
