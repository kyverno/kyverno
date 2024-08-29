package policycache

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceFinder interface {
	FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error)
}

// Cache get method use for to get policy names and mostly use to test cache testcases
type Cache interface {
	// Set inserts a policy in the cache
	Set(string, kyvernov1.PolicyInterface, ResourceFinder) error
	// Unset removes a policy from the cache
	Unset(string)
	// GetPolicies returns all policies that apply to a namespace, including cluster-wide policies
	// If the namespace is empty, only cluster-wide policies are returned
	GetPolicies(PolicyType, schema.GroupVersionResource, string, string) []kyvernov1.PolicyInterface
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

func (c *cache) Set(key string, policy kyvernov1.PolicyInterface, client ResourceFinder) error {
	return c.store.set(key, policy, client)
}

func (c *cache) Unset(key string) {
	c.store.unset(key)
}

func (c *cache) GetPolicies(pkey PolicyType, gvr schema.GroupVersionResource, subresource string, nspace string) []kyvernov1.PolicyInterface {
	var result []kyvernov1.PolicyInterface
	result = append(result, c.store.get(pkey, gvr, subresource, "")...)
	if nspace != "" {
		result = append(result, c.store.get(pkey, gvr, subresource, nspace)...)
	}
	// also get policies with ValidateEnforce
	if pkey == ValidateAudit {
		result = append(result, c.store.get(ValidateEnforce, gvr, subresource, "")...)
	}
	if pkey == ValidateAudit || pkey == ValidateEnforce {
		result = filterPolicies(pkey, result, nspace)
	}
	return result
}

// Filter cluster policies using validationFailureAction override
func filterPolicies(pkey PolicyType, result []kyvernov1.PolicyInterface, nspace string) []kyvernov1.PolicyInterface {
	var policies []kyvernov1.PolicyInterface
	for _, policy := range result {
		var filteredPolicy kyvernov1.PolicyInterface
		keepPolicy := true
		switch pkey {
		case ValidateAudit:
			keepPolicy, filteredPolicy = checkValidationFailureActionOverrides(false, nspace, policy)
		case ValidateEnforce:
			keepPolicy, filteredPolicy = checkValidationFailureActionOverrides(true, nspace, policy)
		}
		// add policy to result
		if keepPolicy {
			policies = append(policies, filteredPolicy)
		}
	}
	return policies
}

func checkValidationFailureActionOverrides(enforce bool, ns string, policy kyvernov1.PolicyInterface) (bool, kyvernov1.PolicyInterface) {
	var filteredRules []kyvernov1.Rule
	for _, rule := range autogen.ComputeRules(policy, "") {
		if !rule.HasValidate() {
			continue
		}

		// if the field isn't set, use the higher level policy setting
		validationFailureAction := rule.Validation.FailureAction
		if validationFailureAction == nil {
			validationFailureAction = &policy.GetSpec().ValidationFailureAction
		}

		validationFailureActionOverrides := rule.Validation.FailureActionOverrides
		if len(validationFailureActionOverrides) == 0 {
			validationFailureActionOverrides = policy.GetSpec().ValidationFailureActionOverrides
		}

		if (ns == "" || len(validationFailureActionOverrides) == 0) && validationFailureAction.Enforce() == enforce {
			filteredRules = append(filteredRules, rule)
			continue
		}
		for _, action := range validationFailureActionOverrides {
			if action.Action.Enforce() == enforce && wildcard.CheckPatterns(action.Namespaces, ns) {
				filteredRules = append(filteredRules, rule)
				continue
			}
		}
	}
	if len(filteredRules) > 0 {
		filteredPolicy := policy.CreateDeepCopy()
		filteredPolicy.GetSpec().Rules = filteredRules
		return true, filteredPolicy
	}

	return false, nil
}
