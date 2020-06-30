package policycache

// package main

import (
	"fmt"
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
	m := pc.pMap.get(ValidateAudit)
	fmt.Println("====got m==", len(m))
	pc.pMap.add(policy)
}

// Get the list of matched policies
func (pc *policyCache) Get(pkey PolicyType) []*kyverno.ClusterPolicy {
	return pc.pMap.get(pkey)
}

// Remove a policy from cache
func (pc *policyCache) Remove(policy *kyverno.ClusterPolicy) {
	pc.pMap.remove(policy)
}

func (m *pMap) add(policy *kyverno.ClusterPolicy) {
	m.Lock()
	defer m.Unlock()

	enforcePolicy := policy.Spec.ValidationFailureAction == "enforce"

	for _, rule := range policy.Spec.Rules {
		if rule.HasMutate() {
			mutatePolicy := m.dataMap[Mutate]
			m.dataMap[Mutate] = append(mutatePolicy, policy)
		}

		if rule.HasValidate() {
			if enforcePolicy {
				validatePolicy := m.dataMap[ValidateEnforce]
				m.dataMap[ValidateEnforce] = append(validatePolicy, policy)
			} else {
				validatePolicy := m.dataMap[ValidateAudit]
				m.dataMap[ValidateAudit] = append(validatePolicy, policy)
			}
		}

		if rule.HasGenerate() {
			generatePolicy := m.dataMap[Generate]
			m.dataMap[Generate] = append(generatePolicy, policy)
		}
	}

	fmt.Println("==add===new dataMap====", m.dataMap)
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

	fmt.Println("===remove==new dataMap====", m.dataMap)
}
