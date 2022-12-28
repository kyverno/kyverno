package policycache

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
)

// Cache get method use for to get policy names and mostly use to test cache testcases
type Cache interface {
	// Set inserts a policy in the cache
	Set(string, kyvernov1.PolicyInterface, map[string]string)
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

func (c *cache) Set(key string, policy kyvernov1.PolicyInterface, subresourceGVKToKind map[string]string) {
	c.store.set(key, policy, subresourceGVKToKind)
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

	if pkey == ValidateAudit { // also get policies with ValidateEnforce
		result = append(result, c.store.get(ValidateEnforce, kind, "")...)
		result = append(result, c.store.get(ValidateEnforce, "*", "")...)
	}

	if pkey == ValidateAudit || pkey == ValidateEnforce {
		result = filterPolicies(pkey, result, nspace, kind)
	}

	return result
}

// Filter cluster policies using validationFailureAction override
func filterPolicies(pkey PolicyType, result []kyvernov1.PolicyInterface, nspace, kind string) []kyvernov1.PolicyInterface {
	var policies []kyvernov1.PolicyInterface
	for _, policy := range result {
		keepPolicy := true
		switch pkey {
		case ValidateAudit:
			keepPolicy = checkValidationFailureActionOverrides(false, nspace, policy)
		case ValidateEnforce:
			keepPolicy = checkValidationFailureActionOverrides(true, nspace, policy)
		}
		if keepPolicy { // add policy to result
			policies = append(policies, policy)
		}
	}
	return policies
}

func checkValidationFailureActionOverrides(enforce bool, ns string, policy kyvernov1.PolicyInterface) bool {
	validationFailureAction := policy.GetSpec().ValidationFailureAction
	validationFailureActionOverrides := policy.GetSpec().ValidationFailureActionOverrides
	if validationFailureAction.Enforce() != enforce && (ns == "" || len(validationFailureActionOverrides) == 0) {
		return false
	}
	for _, action := range validationFailureActionOverrides {
		if action.Action.Enforce() != enforce && wildcard.CheckPatterns(action.Namespaces, ns) {
			return false
		}
	}
	return true
}
