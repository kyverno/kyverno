package policycache

import (
	"strings"
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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

	// Add adds a policy to the cache
	Add(policy *kyverno.ClusterPolicy)

	// Remove removes a policy from the cache
	Remove(policy *kyverno.ClusterPolicy)

	// GetPolicies returns all policies that apply to a namespace, including cluster-wide policies
	// If the namespace is empty, only cluster-wide policies are returned
	GetPolicies(pkey PolicyType, kind string, nspace string) []*kyverno.ClusterPolicy

	get(pkey PolicyType, kind string, nspace string) []string
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
func (pc *policyCache) Add(policy *kyverno.ClusterPolicy) {
	pc.pMap.add(policy)
	pc.Logger.V(4).Info("policy is added to cache", "name", policy.GetName())
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
	imageVerifyMap := m.nameCacheMap[VerifyImages]

	var pName = policy.GetName()
	pSpace := policy.GetNamespace()
	if pSpace != "" {
		pName = pSpace + "/" + pName
	}

	for _, rule := range policy.Spec.Rules {

		if len(rule.MatchResources.Any) > 0 {
			for _, rmr := range rule.MatchResources.Any {
				addCacheHelper(rmr, m, rule, mutateMap, pName, enforcePolicy, validateEnforceMap, validateAuditMap, generateMap, imageVerifyMap)
			}
		} else if len(rule.MatchResources.All) > 0 {
			for _, rmr := range rule.MatchResources.All {
				addCacheHelper(rmr, m, rule, mutateMap, pName, enforcePolicy, validateEnforceMap, validateAuditMap, generateMap, imageVerifyMap)
			}
		} else {
			r := kyverno.ResourceFilter{UserInfo: rule.MatchResources.UserInfo, ResourceDescription: rule.MatchResources.ResourceDescription}
			addCacheHelper(r, m, rule, mutateMap, pName, enforcePolicy, validateEnforceMap, validateAuditMap, generateMap, imageVerifyMap)
		}
	}

	m.nameCacheMap[Mutate] = mutateMap
	m.nameCacheMap[ValidateEnforce] = validateEnforceMap
	m.nameCacheMap[ValidateAudit] = validateAuditMap
	m.nameCacheMap[Generate] = generateMap
	m.nameCacheMap[VerifyImages] = imageVerifyMap
}

func addCacheHelper(rmr kyverno.ResourceFilter, m *pMap, rule kyverno.Rule, mutateMap map[string]bool, pName string, enforcePolicy bool, validateEnforceMap map[string]bool, validateAuditMap map[string]bool, generateMap map[string]bool, imageVerifyMap map[string]bool) {
	for _, gvk := range rmr.Kinds {
		_, k := common.GetKindFromGVK(gvk)
		kind := strings.Title(k)
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

		if rule.HasVerifyImages() {
			if !imageVerifyMap[kind+"/"+pName] {
				imageVerifyMap[kind+"/"+pName] = true
				imageVerifyMapPolicy := m.kindDataMap[kind][VerifyImages]
				m.kindDataMap[kind][VerifyImages] = append(imageVerifyMapPolicy, pName)
			}
			continue
		}
	}
}

func (m *pMap) get(key PolicyType, gvk, namespace string) (names []string) {
	m.RLock()
	defer m.RUnlock()
	_, kind := common.GetKindFromGVK(gvk)
	for _, policyName := range m.kindDataMap[kind][key] {
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

		if len(rule.MatchResources.Any) > 0 {
			for _, rmr := range rule.MatchResources.Any {
				removeCacheHelper(rmr, m, pName)
			}
		} else if len(rule.MatchResources.All) > 0 {
			for _, rmr := range rule.MatchResources.All {
				removeCacheHelper(rmr, m, pName)
			}
		} else {
			r := kyverno.ResourceFilter{UserInfo: rule.MatchResources.UserInfo, ResourceDescription: rule.MatchResources.ResourceDescription}
			removeCacheHelper(r, m, pName)
		}
	}
}

func removeCacheHelper(rmr kyverno.ResourceFilter, m *pMap, pName string) {
	for _, gvk := range rmr.Kinds {
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
