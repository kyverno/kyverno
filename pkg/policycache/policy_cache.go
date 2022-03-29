package policycache

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
)

// Interface ...
// Interface get method use for to get policy names and mostly use to test cache testcases
type Interface interface {
	// GetPolicies returns all policies that apply to a namespace, including cluster-wide policies
	// If the namespace is empty, only cluster-wide policies are returned
	GetPolicies(pkey PolicyType, kind string, nspace string) []*kyverno.ClusterPolicy

	// add adds a policy to the cache
	add(policy *kyverno.ClusterPolicy)

	// remove removes a policy from the cache
	remove(policy *kyverno.ClusterPolicy)

	get(pkey PolicyType, kind string, nspace string) []string
}

// policyCache ...
type policyCache struct {
	pMap   pMap
	logger logr.Logger

	// list/get cluster policy resource
	pLister kyvernolister.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernolister.PolicyLister
}

// newPolicyCache ...
func newPolicyCache(log logr.Logger, pLister kyvernolister.ClusterPolicyLister, npLister kyvernolister.PolicyLister) Interface {
	namesCache := map[PolicyType]map[string]bool{
		Mutate:          make(map[string]bool),
		ValidateEnforce: make(map[string]bool),
		ValidateAudit:   make(map[string]bool),
		Generate:        make(map[string]bool),
		VerifyImages:    make(map[string]bool),
	}

	return &policyCache{
		pMap{
			nameCacheMap: namesCache,
			kindDataMap:  make(map[string]map[PolicyType][]string),
		},
		log,
		pLister,
		npLister,
	}
}

// Add a policy to cache
func (pc *policyCache) add(policy *kyverno.ClusterPolicy) {
	pc.pMap.add(policy)
	pc.logger.V(4).Info("policy is added to cache", "name", policy.GetName())
}

// Get the list of matched policies
func (pc *policyCache) get(pkey PolicyType, kind, nspace string) []string {
	return pc.pMap.get(pkey, kind, nspace)
}

func (pc *policyCache) GetPolicies(pkey PolicyType, kind, nspace string) []*kyverno.ClusterPolicy {
	policies := pc.getPolicyObject(pkey, kind, "")
	if nspace == "" {
		return policies
	}
	nsPolicies := pc.getPolicyObject(pkey, kind, nspace)
	return append(policies, nsPolicies...)
}

// Remove a policy from cache
func (pc *policyCache) remove(policy *kyverno.ClusterPolicy) {
	pc.pMap.remove(policy)
	pc.logger.V(4).Info("policy is removed from cache", "name", policy.GetName())
}

func (pc *policyCache) getPolicyObject(key PolicyType, gvk string, nspace string) (policyObject []*kyverno.ClusterPolicy) {
	_, kind := common.GetKindFromGVK(gvk)
	policyNames := pc.pMap.get(key, kind, nspace)
	wildcardPolicies := pc.pMap.get(key, "*", nspace)
	policyNames = append(policyNames, wildcardPolicies...)
	for _, policyName := range policyNames {
		var policy *kyverno.ClusterPolicy
		ns, key, isNamespacedPolicy := policy2.ParseNamespacedPolicy(policyName)
		if !isNamespacedPolicy {
			policy, _ = pc.pLister.Get(key)
		} else {
			if ns == nspace {
				nspolicy, _ := pc.npLister.Policies(ns).Get(key)
				policy = policy2.ConvertPolicyToClusterPolicy(nspolicy)
			}
		}
		policyObject = append(policyObject, policy)
	}
	return policyObject
}
