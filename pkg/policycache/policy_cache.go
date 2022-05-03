package policycache

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policy"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
)

// Interface ...
// Interface get method use for to get policy names and mostly use to test cache testcases
type Interface interface {
	// GetPolicies returns all policies that apply to a namespace, including cluster-wide policies
	// If the namespace is empty, only cluster-wide policies are returned
	GetPolicies(pType PolicyType, kind string, namespace string) []kyverno.PolicyInterface

	// add adds a policy to the cache
	add(kyverno.PolicyInterface)

	// remove removes a policy from the cache
	remove(kyverno.PolicyInterface)

	// update update a policy from the cache
	update(kyverno.PolicyInterface, kyverno.PolicyInterface)

	get(PolicyType, string, string) []string
}

// policyCache ...
type policyCache struct {
	pMap pMap

	// list/get cluster policy resource
	pLister kyvernolister.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernolister.PolicyLister
}

// newPolicyCache ...
func newPolicyCache(pLister kyvernolister.ClusterPolicyLister, npLister kyvernolister.PolicyLister) Interface {
	namesCache := map[PolicyType]map[string]bool{
		Mutate:               make(map[string]bool),
		ValidateEnforce:      make(map[string]bool),
		ValidateAudit:        make(map[string]bool),
		Generate:             make(map[string]bool),
		VerifyImagesMutate:   make(map[string]bool),
		VerifyImagesValidate: make(map[string]bool),
	}

	return &policyCache{
		pMap{
			nameCacheMap: namesCache,
			kindDataMap:  make(map[string]map[PolicyType][]string),
		},
		pLister,
		npLister,
	}
}

// Add a policy to cache
func (pc *policyCache) add(policy kyverno.PolicyInterface) {
	pc.pMap.add(policy)
	logger.V(4).Info("policy is added to cache", "name", policy.GetName())
}

// Get the list of matched policies
func (pc *policyCache) get(pkey PolicyType, kind, nspace string) []string {
	return pc.pMap.get(pkey, kind, nspace)
}

func (pc *policyCache) GetPolicies(pkey PolicyType, kind, nspace string) []kyverno.PolicyInterface {
	policies := pc.getPolicyObject(pkey, kind, "")
	if nspace == "" {
		return policies
	}
	nsPolicies := pc.getPolicyObject(pkey, kind, nspace)
	return append(policies, nsPolicies...)
}

// Remove a policy from cache
func (pc *policyCache) remove(p kyverno.PolicyInterface) {
	pc.pMap.remove(p)
	logger.V(4).Info("policy is removed from cache", "name", p.GetName())
}

func (pc *policyCache) update(oldP kyverno.PolicyInterface, newP kyverno.PolicyInterface) {
	pc.pMap.update(oldP, newP)
	logger.V(4).Info("policy is updated from cache", "name", newP.GetName())
}

func (pc *policyCache) getPolicyObject(key PolicyType, gvk string, nspace string) (policyObject []kyverno.PolicyInterface) {
	_, kind := kubeutils.GetKindFromGVK(gvk)
	policyNames := pc.pMap.get(key, kind, nspace)
	wildcardPolicies := pc.pMap.get(key, "*", nspace)
	policyNames = append(policyNames, wildcardPolicies...)
	for _, policyName := range policyNames {
		var p kyverno.PolicyInterface
		ns, key, isNamespacedPolicy := policy.ParseNamespacedPolicy(policyName)
		if !isNamespacedPolicy {
			p, _ = pc.pLister.Get(key)
		} else {
			if ns == nspace {
				p, _ = pc.npLister.Policies(ns).Get(key)
			}
		}
		if p != nil {
			policyObject = append(policyObject, p)
		}
	}
	return policyObject
}
