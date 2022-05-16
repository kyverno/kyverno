package policycache

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

// Cache get method use for to get policy names and mostly use to test cache testcases
type Cache interface {
	// Set inserts a policy in the cache
	Set(string, kyvernov1.PolicyInterface)
	// Unset removes a policy from the cache
	Unset(string)
	// GetPolicies returns all policies that apply to a namespace, including cluster-wide policies
	// If the namespace is empty, only cluster-wide policies are returned
	GetPolicies(PolicyType, string, string) []kyvernov1.PolicyInterface
}

type cache struct {
	store store
}

// NewCache create a new Cache
func NewCache() Cache {
	return &cache{
		store: newPolicyCache(),
	}
}

func (c *cache) Set(key string, policy kyvernov1.PolicyInterface) {
	c.store.set(key, policy)
}

func (c *cache) Unset(key string) {
	c.store.unset(key)
}

func (c *cache) GetPolicies(pkey PolicyType, kind, nspace string) []kyvernov1.PolicyInterface {
	var result []kyvernov1.PolicyInterface
	result = append(result, c.store.get(pkey, kind, "")...)
	result = append(result, c.store.get(pkey, "*", "")...)
	if nspace != "" {
		result = append(result, c.store.get(pkey, kind, nspace)...)
		result = append(result, c.store.get(pkey, "*", nspace)...)
	}
	return result
}
