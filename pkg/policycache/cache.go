package policycache

import (
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
)

type pMap struct {
	sync.RWMutex

	// kindDataMap field stores names of ClusterPolicies and  Namespaced Policies.
	// Since both the policy name use same type (i.e. string), Both policies can be differentiated based on
	// "namespace". namespace policy get stored with policy namespace with policy name"
	// kindDataMap {"kind": {{"policytype" : {"policyName","nsname/policyName}}},"kind2": {{"policytype" : {"nsname/policyName" }}}}
	kindDataMap map[string]map[PolicyType][]string

	// nameCacheMap stores the names of all existing policies in dataMap
	// Policy names are stored as <namespace>/<name>
	nameCacheMap map[PolicyType]map[string]bool
}

// policyCache ...
type policyCache struct {
	pMap
	logr.Logger
	// list/get cluster policy resource
	pLister kyvernolister.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernolister.PolicyLister
}

// Interface ...
// Interface get method use for to get policy names and mostly use to test cache testcases
type Interface interface {
	Add(policy *kyverno.ClusterPolicy)
	Remove(policy *kyverno.ClusterPolicy)
	GetPolicyObject(pkey PolicyType, kind string, nspace string) []*kyverno.ClusterPolicy
	get(pkey PolicyType, kind string, nspace string) []string
}

// newPolicyCache ...
func newPolicyCache(log logr.Logger, pLister kyvernolister.ClusterPolicyLister, npLister kyvernolister.PolicyLister) Interface {
	namesCache := map[PolicyType]map[string]bool{
		Mutate:          make(map[string]bool),
		ValidateEnforce: make(map[string]bool),
		ValidateAudit:   make(map[string]bool),
		Generate:        make(map[string]bool),
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
func (pc *policyCache) Add(policy *kyverno.ClusterPolicy) {
	pc.pMap.add(policy)
	pc.Logger.V(4).Info("policy is added to cache", "name", policy.GetName())
}

// Get the list of matched policies
func (pc *policyCache) get(pkey PolicyType, kind, nspace string) []string {
	return pc.pMap.get(pkey, kind, nspace)
}
func (pc *policyCache) GetPolicyObject(pkey PolicyType, kind, nspace string) []*kyverno.ClusterPolicy {
	return pc.getPolicyObject(pkey, kind, nspace)
}

// Remove a policy from cache
func (pc *policyCache) Remove(policy *kyverno.ClusterPolicy) {
	pc.pMap.remove(policy)
	pc.Logger.V(4).Info("policy is removed from cache", "name", policy.GetName())
}

func (m *pMap) add(policy *kyverno.ClusterPolicy) {
	m.Lock()
	defer m.Unlock()

	enforcePolicy := policy.Spec.ValidationFailureAction == "enforce"
	mutateMap := m.nameCacheMap[Mutate]
	validateEnforceMap := m.nameCacheMap[ValidateEnforce]
	validateAuditMap := m.nameCacheMap[ValidateAudit]
	generateMap := m.nameCacheMap[Generate]
	var pName = policy.GetName()
	pSpace := policy.GetNamespace()
	if pSpace != "" {
		pName = pSpace + "/" + pName
	}
	for _, rule := range policy.Spec.Rules {

		for _, gvk := range rule.MatchResources.Kinds {
			_, kind := common.GetKindFromGVK(gvk)
			_, ok := m.kindDataMap[kind]
			if !ok {
				m.kindDataMap[kind] = make(map[PolicyType][]string)
			}

			if rule.HasMutate() {
				if !mutateMap[kind+"/"+pName] {
					mutateMap[kind+"/"+pName] = true
					mutatePolicy := m.kindDataMap[kind][Mutate]
					m.kindDataMap[kind][Mutate] = append(mutatePolicy, pName)
				}
				continue
			}
			if rule.HasValidate() {
				if enforcePolicy {
					if !validateEnforceMap[kind+"/"+pName] {
						validateEnforceMap[kind+"/"+pName] = true
						validatePolicy := m.kindDataMap[kind][ValidateEnforce]
						m.kindDataMap[kind][ValidateEnforce] = append(validatePolicy, pName)
					}
					continue
				}

				// ValidateAudit
				if !validateAuditMap[kind+"/"+pName] {
					validateAuditMap[kind+"/"+pName] = true
					validatePolicy := m.kindDataMap[kind][ValidateAudit]
					m.kindDataMap[kind][ValidateAudit] = append(validatePolicy, pName)
				}
				continue
			}

			if rule.HasGenerate() {
				if !generateMap[kind+"/"+pName] {
					generateMap[kind+"/"+pName] = true
					generatePolicy := m.kindDataMap[kind][Generate]
					m.kindDataMap[kind][Generate] = append(generatePolicy, pName)
				}
				continue
			}
		}
	}
	m.nameCacheMap[Mutate] = mutateMap
	m.nameCacheMap[ValidateEnforce] = validateEnforceMap
	m.nameCacheMap[ValidateAudit] = validateAuditMap
	m.nameCacheMap[Generate] = generateMap
}

func (pc *pMap) get(key PolicyType, gvk, namespace string) (names []string) {
	pc.RLock()
	defer pc.RUnlock()
	_, kind := common.GetKindFromGVK(gvk)
	for _, policyName := range pc.kindDataMap[kind][key] {
		ns, key, isNamespacedPolicy := policy2.ParseNamespacedPolicy(policyName)
		if !isNamespacedPolicy && namespace == "" {
			names = append(names, key)
		} else {
			if ns == namespace {
				names = append(names, policyName)
			}
		}
	}
	return names
}

func (m *pMap) remove(policy *kyverno.ClusterPolicy) {
	m.Lock()
	defer m.Unlock()
	var pName = policy.GetName()
	pSpace := policy.GetNamespace()
	if pSpace != "" {
		pName = pSpace + "/" + pName
	}

	for _, rule := range policy.Spec.Rules {
		for _, gvk := range rule.MatchResources.Kinds {
			_, kind := common.GetKindFromGVK(gvk)
			dataMap := m.kindDataMap[kind]
			for policyType, policies := range dataMap {
				var newPolicies []string
				for _, p := range policies {
					if p == pName {
						continue
					}
					newPolicies = append(newPolicies, p)
				}
				m.kindDataMap[kind][policyType] = newPolicies
			}
			for _, nameCache := range m.nameCacheMap {
				if ok := nameCache[kind+"/"+pName]; ok {
					delete(nameCache, kind+"/"+pName)
				}
			}

		}
	}
}
func (m *policyCache) getPolicyObject(key PolicyType, gvk string, nspace string) (policyObject []*kyverno.ClusterPolicy) {
	_, kind := common.GetKindFromGVK(gvk)
	policyNames := m.pMap.get(key, kind, nspace)
	for _, policyName := range policyNames {
		var policy *kyverno.ClusterPolicy
		ns, key, isNamespacedPolicy := policy2.ParseNamespacedPolicy(policyName)
		if !isNamespacedPolicy {
			policy, _ = m.pLister.Get(key)
		} else {
			if ns == nspace {
				nspolicy, _ := m.npLister.Policies(ns).Get(key)
				policy = policy2.ConvertPolicyToClusterPolicy(nspolicy)
			}
		}
		policyObject = append(policyObject, policy)
	}
	return policyObject
}
