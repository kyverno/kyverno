package policycache

import (
	"sync"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/policy"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
)

type pMap struct {
	lock sync.RWMutex

	// kindDataMap field stores names of ClusterPolicies and  Namespaced Policies.
	// Since both the policy name use same type (i.e. string), Both policies can be differentiated based on
	// "namespace". namespace policy get stored with policy namespace with policy name"
	// kindDataMap {"kind": {{"policytype" : {"policyName","nsname/policyName}}},"kind2": {{"policytype" : {"nsname/policyName" }}}}
	kindDataMap map[string]map[PolicyType][]string

	// nameCacheMap stores the names of all existing policies in dataMap
	// Policy names are stored as <namespace>/<name>
	nameCacheMap map[PolicyType]map[string]bool
}

func (m *pMap) add(policy kyverno.PolicyInterface) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.addPolicyToCache(policy)
}
func (m *pMap) addPolicyToCache(policy kyverno.PolicyInterface) {
	spec := policy.GetSpec()
	enforcePolicy := spec.GetValidationFailureAction() == kyverno.Enforce
	for _, k := range spec.ValidationFailureActionOverrides {
		if k.Action == kyverno.Enforce {
			enforcePolicy = true
			break
		}
	}

	var pName = policy.GetName()
	pSpace := policy.GetNamespace()
	if pSpace != "" {
		pName = pSpace + "/" + pName
	}

	for _, rule := range autogen.ComputeRules(policy) {
		if len(rule.MatchResources.Any) > 0 {
			for _, rmr := range rule.MatchResources.Any {
				addCacheHelper(rmr, m, rule, pName, enforcePolicy)
			}
		} else if len(rule.MatchResources.All) > 0 {
			for _, rmr := range rule.MatchResources.All {
				addCacheHelper(rmr, m, rule, pName, enforcePolicy)
			}
		} else {
			r := kyverno.ResourceFilter{UserInfo: rule.MatchResources.UserInfo, ResourceDescription: rule.MatchResources.ResourceDescription}
			addCacheHelper(r, m, rule, pName, enforcePolicy)
		}
	}
}

func (m *pMap) get(key PolicyType, gvk, namespace string) (names []string) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	_, kind := kubeutils.GetKindFromGVK(gvk)
	for _, policyName := range m.kindDataMap[kind][key] {
		ns, key, isNamespacedPolicy := policy.ParseNamespacedPolicy(policyName)
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

func (m *pMap) remove(policy kyverno.PolicyInterface) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removePolicyFromCache(policy)
}
func (m *pMap) removePolicyFromCache(policy kyverno.PolicyInterface) {
	var pName = policy.GetName()
	pSpace := policy.GetNamespace()
	if pSpace != "" {
		pName = pSpace + "/" + pName
	}

	for _, rule := range autogen.ComputeRules(policy) {
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

func (m *pMap) update(old kyverno.PolicyInterface, new kyverno.PolicyInterface) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removePolicyFromCache(old)
	m.addPolicyToCache(new)
}

func addCacheHelper(rmr kyverno.ResourceFilter, m *pMap, rule kyverno.Rule, pName string, enforcePolicy bool) {
	for _, gvk := range rmr.Kinds {
		_, k := kubeutils.GetKindFromGVK(gvk)
		kind, _ := kubeutils.SplitSubresource(k)
		_, ok := m.kindDataMap[kind]
		if !ok {
			m.kindDataMap[kind] = make(map[PolicyType][]string)
		}

		if rule.HasMutate() {
			if !m.nameCacheMap[Mutate][kind+"/"+pName] {
				m.nameCacheMap[Mutate][kind+"/"+pName] = true
				mutatePolicy := m.kindDataMap[kind][Mutate]
				m.kindDataMap[kind][Mutate] = append(mutatePolicy, pName)
			}

			continue
		}

		if rule.HasValidate() {
			if enforcePolicy {
				if !m.nameCacheMap[ValidateEnforce][kind+"/"+pName] {
					m.nameCacheMap[ValidateEnforce][kind+"/"+pName] = true
					validatePolicy := m.kindDataMap[kind][ValidateEnforce]
					m.kindDataMap[kind][ValidateEnforce] = append(validatePolicy, pName)
				}

				continue
			}

			if !m.nameCacheMap[ValidateAudit][kind+"/"+pName] {
				m.nameCacheMap[ValidateAudit][kind+"/"+pName] = true
				validatePolicy := m.kindDataMap[kind][ValidateAudit]
				m.kindDataMap[kind][ValidateAudit] = append(validatePolicy, pName)
			}

			continue
		}

		if rule.HasGenerate() {
			if !m.nameCacheMap[Generate][kind+"/"+pName] {
				m.nameCacheMap[Generate][kind+"/"+pName] = true
				generatePolicy := m.kindDataMap[kind][Generate]
				m.kindDataMap[kind][Generate] = append(generatePolicy, pName)
			}

			continue
		}

		if rule.HasVerifyImages() {
			if !m.nameCacheMap[VerifyImagesMutate][kind+"/"+pName] {
				m.nameCacheMap[VerifyImagesMutate][kind+"/"+pName] = true
				imageVerifyMapPolicy := m.kindDataMap[kind][VerifyImagesMutate]
				m.kindDataMap[kind][VerifyImagesMutate] = append(imageVerifyMapPolicy, pName)
			}

			if rule.HasImagesValidationChecks() {
				m.nameCacheMap[VerifyImagesValidate][kind+"/"+pName] = true
				imageVerifyMapPolicy := m.kindDataMap[kind][VerifyImagesValidate]
				m.kindDataMap[kind][VerifyImagesValidate] = append(imageVerifyMapPolicy, pName)
			}

			continue
		}
	}
}

func removeCacheHelper(rmr kyverno.ResourceFilter, m *pMap, pName string) {
	for _, gvk := range rmr.Kinds {
		_, kind := kubeutils.GetKindFromGVK(gvk)
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
