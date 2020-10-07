package policycache

import (
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
)

type pMap struct {
	sync.RWMutex
	// dataMap field stores ClusterPolicies
	dataMap map[PolicyType][]*kyverno.ClusterPolicy
	// nsDataMap field stores Namespaced Policies for each namespaces.
	// The Policy is converted internally to ClusterPolicy and stored as a ClusterPolicy
	// Since both the policy use same type (i.e. Policy), Both policies can be differentiated based on
	// "Kind" or "namespace". When the Policy is converted it will retain the value of kind as "Policy".
	// Cluster policy will be having namespace as Blank (""), but Policy will always be having namespace field and "default" value by default
	nsDataMap map[string]map[PolicyType][]*kyverno.ClusterPolicy

	// nameCacheMap stores the names of all existing policies in dataMap
	// Policy names are stored as <namespace>/<name>
	nameCacheMap map[PolicyType]map[string]bool
}

// policyCache ...
type policyCache struct {
	pMap
	logr.Logger
}

// Interface ...
type Interface interface {
	Add(policy *kyverno.ClusterPolicy)
	Remove(policy *kyverno.ClusterPolicy)
	Get(pkey PolicyType, nspace *string) []*kyverno.ClusterPolicy
}

// newPolicyCache ...
func newPolicyCache(log logr.Logger) Interface {
	namesCache := map[PolicyType]map[string]bool{
		Mutate:          make(map[string]bool),
		ValidateEnforce: make(map[string]bool),
		ValidateAudit:   make(map[string]bool),
		Generate:        make(map[string]bool),
	}

	return &policyCache{
		pMap{
			dataMap:      make(map[PolicyType][]*kyverno.ClusterPolicy),
			nsDataMap:    make(map[string]map[PolicyType][]*kyverno.ClusterPolicy),
			nameCacheMap: namesCache,
		},
		log,
	}
}

// Add a policy to cache
func (pc *policyCache) Add(policy *kyverno.ClusterPolicy) {
	pc.pMap.add(policy)

	pc.Logger.V(4).Info("policy is added to cache", "name", policy.GetName())
}

// Get the list of matched policies
func (pc *policyCache) Get(pkey PolicyType, nspace *string) []*kyverno.ClusterPolicy {
	return pc.pMap.get(pkey, nspace)
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
	isNamespacedPolicy := false
	if pSpace != "" {
		pName = pSpace + "/" + pName
		isNamespacedPolicy = true
		// Initialize Namespace Cache Map
		_, ok := m.nsDataMap[policy.GetNamespace()]
		if !ok {
			m.nsDataMap[policy.GetNamespace()] = make(map[PolicyType][]*kyverno.ClusterPolicy)
		}
	}

	for _, rule := range policy.Spec.Rules {
		if rule.HasMutate() {
			if !mutateMap[pName] {
				mutateMap[pName] = true
				if isNamespacedPolicy {
					mutatePolicy := m.nsDataMap[policy.GetNamespace()][Mutate]
					m.nsDataMap[policy.GetNamespace()][Mutate] = append(mutatePolicy, policy)
					continue
				}
				mutatePolicy := m.dataMap[Mutate]
				m.dataMap[Mutate] = append(mutatePolicy, policy)
			}
			continue
		}

		if rule.HasValidate() {
			if enforcePolicy {
				if !validateEnforceMap[pName] {
					validateEnforceMap[pName] = true
					if isNamespacedPolicy {
						validatePolicy := m.nsDataMap[policy.GetNamespace()][ValidateEnforce]
						m.nsDataMap[policy.GetNamespace()][ValidateEnforce] = append(validatePolicy, policy)
						continue
					}
					validatePolicy := m.dataMap[ValidateEnforce]
					m.dataMap[ValidateEnforce] = append(validatePolicy, policy)
				}
				continue
			}

			// ValidateAudit
			if !validateAuditMap[pName] {
				validateAuditMap[pName] = true
				if isNamespacedPolicy {
					validatePolicy := m.nsDataMap[policy.GetNamespace()][ValidateAudit]
					m.nsDataMap[policy.GetNamespace()][ValidateAudit] = append(validatePolicy, policy)
					continue
				}
				validatePolicy := m.dataMap[ValidateAudit]
				m.dataMap[ValidateAudit] = append(validatePolicy, policy)
			}
			continue
		}

		if rule.HasGenerate() {
			if !generateMap[pName] {
				generateMap[pName] = true
				if isNamespacedPolicy {
					generatePolicy := m.nsDataMap[policy.GetNamespace()][Generate]
					m.nsDataMap[policy.GetNamespace()][Generate] = append(generatePolicy, policy)
					continue
				}
				generatePolicy := m.dataMap[Generate]
				m.dataMap[Generate] = append(generatePolicy, policy)
			}
			continue
		}
	}

	m.nameCacheMap[Mutate] = mutateMap
	m.nameCacheMap[ValidateEnforce] = validateEnforceMap
	m.nameCacheMap[ValidateAudit] = validateAuditMap
	m.nameCacheMap[Generate] = generateMap
}

func (m *pMap) get(key PolicyType, nspace *string) []*kyverno.ClusterPolicy {
	m.RLock()
	defer m.RUnlock()
	if nspace == nil || *nspace == "" {
		return m.dataMap[key]
	}
	return m.nsDataMap[*nspace][key]

}

func (m *pMap) remove(policy *kyverno.ClusterPolicy) {
	m.Lock()
	defer m.Unlock()

	var pName = policy.GetName()
	pSpace := policy.GetNamespace()
	isNamespacedPolicy := false
	if pSpace != "" {
		pName = pSpace + "/" + pName
		isNamespacedPolicy = true
	}
	if !isNamespacedPolicy {
		dataMap := m.dataMap
		for k, policies := range dataMap {

			var newPolicies []*kyverno.ClusterPolicy
			for _, p := range policies {
				if p.GetName() == pName {
					continue
				}
				newPolicies = append(newPolicies, p)
			}

			m.dataMap[k] = newPolicies
		}
	} else {
		dataMap := m.nsDataMap[pSpace]
		for k, policies := range dataMap {

			var newPolicies []*kyverno.ClusterPolicy
			for _, p := range policies {
				if (p.GetNamespace() + "/" + p.GetName()) == pName {
					continue
				}
				newPolicies = append(newPolicies, p)
			}

			m.nsDataMap[pSpace][k] = newPolicies
		}
	}

	for _, nameCache := range m.nameCacheMap {
		if _, ok := nameCache[pName]; ok {
			delete(nameCache, pName)
		}
	}
}
