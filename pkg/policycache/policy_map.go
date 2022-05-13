package policycache

import (
	"sync"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/policy"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/util/sets"
)

type pMap struct {
	lock sync.RWMutex

	// kindDataMap field stores names of ClusterPolicies and  Namespaced Policies.
	// Since both the policy name use same type (i.e. string), Both policies can be differentiated based on
	// "namespace". namespace policy get stored with policy namespace with policy name"
	// kindDataMap {"kind": {{"policytype" : {"policyName","nsname/policyName}}},"kind2": {{"policytype" : {"nsname/policyName" }}}}
	kindDataMap map[string]map[PolicyType]sets.String
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
		addCacheHelper(rule.MatchResources, m, rule, pName, enforcePolicy)
	}
}

func (m *pMap) get(key PolicyType, gvk, namespace string) (names []string) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	_, kind := kubeutils.GetKindFromGVK(gvk)
	for policyName := range m.kindDataMap[kind][key] {
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
		removeCacheHelper(rule.MatchResources, m, pName)
	}
}

func (m *pMap) update(old kyverno.PolicyInterface, new kyverno.PolicyInterface) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removePolicyFromCache(old)
	m.addPolicyToCache(new)
}

func getKind(gvk string) string {
	_, k := kubeutils.GetKindFromGVK(gvk)
	kind, _ := kubeutils.SplitSubresource(k)
	return kind
}

func addCacheHelper(match kyverno.MatchResources, m *pMap, rule kyverno.Rule, pName string, enforcePolicy bool) {
	for _, gvk := range match.GetKinds() {
		kind := getKind(gvk)
		if m.kindDataMap[kind] == nil {
			m.kindDataMap[kind] = map[PolicyType]sets.String{
				Mutate:               sets.NewString(),
				ValidateEnforce:      sets.NewString(),
				ValidateAudit:        sets.NewString(),
				Generate:             sets.NewString(),
				VerifyImagesMutate:   sets.NewString(),
				VerifyImagesValidate: sets.NewString(),
			}
		}
		if rule.HasMutate() {
			m.kindDataMap[kind][Mutate] = m.kindDataMap[kind][Mutate].Insert(pName)
			continue
		}
		if rule.HasValidate() {
			if enforcePolicy {
				m.kindDataMap[kind][ValidateEnforce] = m.kindDataMap[kind][ValidateEnforce].Insert(pName)
			} else {
				m.kindDataMap[kind][ValidateAudit] = m.kindDataMap[kind][ValidateAudit].Insert(pName)
			}
			continue
		}
		if rule.HasGenerate() {
			m.kindDataMap[kind][Generate] = m.kindDataMap[kind][Generate].Insert(pName)
			continue
		}
		if rule.HasVerifyImages() {
			m.kindDataMap[kind][VerifyImagesMutate] = m.kindDataMap[kind][VerifyImagesMutate].Insert(pName)
			if rule.HasImagesValidationChecks() {
				m.kindDataMap[kind][VerifyImagesValidate] = m.kindDataMap[kind][VerifyImagesValidate].Insert(pName)
			}
			continue
		}
	}
}

func removeCacheHelper(match kyverno.MatchResources, m *pMap, pName string) {
	for _, gvk := range match.GetKinds() {
		kind := getKind(gvk)
		for policyType, policies := range m.kindDataMap[kind] {
			m.kindDataMap[kind][policyType] = policies.Delete(pName)
		}
	}
}
