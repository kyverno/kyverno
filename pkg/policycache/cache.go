package policycache

// package main

import (
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type pMap struct {
	sync.RWMutex
	dataMap map[PolicyType][]*kyverno.ClusterPolicy
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
	Get(pkey PolicyType) []*kyverno.ClusterPolicy
}

// newPolicyCache ...
func newPolicyCache(log logr.Logger) Interface {
	return &policyCache{
		pMap{
			dataMap: make(map[PolicyType][]*kyverno.ClusterPolicy),
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
func (pc *policyCache) Get(pkey PolicyType) []*kyverno.ClusterPolicy {
	return pc.pMap.get(pkey)
}

// Remove a policy from cache
func (pc *policyCache) Remove(policy *kyverno.ClusterPolicy) {
	pc.pMap.remove(policy)
	pc.Logger.V(4).Info("policy is removed from cache", "name", policy.GetName())
}

// buildCacheMap builds the map to store the names all existing
// policies in the cache, it is used to aviod adding duplicate policies
func buildCacheMap(policies []*kyverno.ClusterPolicy) map[string]bool {
	cacheMap := make(map[string]bool)

	for _, p := range policies {
		name := p.GetName()
		if !cacheMap[name] {
			cacheMap[p.GetName()] = true
		}
	}

	return cacheMap
}

func (m *pMap) add(policy *kyverno.ClusterPolicy) {
	m.Lock()
	defer m.Unlock()

	enforcePolicy := policy.Spec.ValidationFailureAction == "enforce"
	mutateMap := buildCacheMap(m.dataMap[Mutate])
	validateMap := buildCacheMap(m.dataMap[ValidateEnforce])
	generateMap := buildCacheMap(m.dataMap[Generate])

	pName := policy.GetName()
	for _, rule := range policy.Spec.Rules {
		if rule.HasMutate() {
			if !mutateMap[pName] {
				mutateMap[pName] = true

				mutatePolicy := m.dataMap[Mutate]
				m.dataMap[Mutate] = append(mutatePolicy, policy)
			}
			continue
		}

		if rule.HasValidate() && enforcePolicy {
			if !validateMap[pName] {
				validateMap[pName] = true

				validatePolicy := m.dataMap[ValidateEnforce]
				m.dataMap[ValidateEnforce] = append(validatePolicy, policy)
			}
			continue
		}

		if rule.HasGenerate() {
			if !generateMap[pName] {
				generateMap[pName] = true

				generatePolicy := m.dataMap[Generate]
				m.dataMap[Generate] = append(generatePolicy, policy)
			}
			continue
		}
	}
}

func (m *pMap) get(key PolicyType) []*kyverno.ClusterPolicy {
	m.RLock()
	defer m.RUnlock()

	return m.dataMap[key]
}

func (m *pMap) remove(policy *kyverno.ClusterPolicy) {
	m.Lock()
	defer m.Unlock()

	dataMap := m.dataMap
	for k, policies := range dataMap {

		var newPolicies []*kyverno.ClusterPolicy
		for _, p := range policies {
			if p.GetName() == policy.GetName() {
				continue
			}
			newPolicies = append(newPolicies, p)
		}

		m.dataMap[k] = newPolicies
	}
}
