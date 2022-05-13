package policycache

import (
	"sync"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/policy"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/util/sets"
)

type store interface {
	// add adds a policy to the cache
	add(kyverno.PolicyInterface)
	// remove removes a policy from the cache
	remove(kyverno.PolicyInterface)
	// update update a policy from the cache
	update(kyverno.PolicyInterface, kyverno.PolicyInterface)
	// get finds policies that match a given type, gvk and namespace
	get(PolicyType, string, string) []string
}

type policyCache struct {
	store
	lock sync.RWMutex
}

func newPolicyCache() store {
	return &policyCache{
		store: policyMap{},
	}
}

func (pc *policyCache) add(policy kyverno.PolicyInterface) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	pc.store.add(policy)
	logger.V(4).Info("policy is added to cache", "name", policy.GetName())
}

func (pc *policyCache) remove(p kyverno.PolicyInterface) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	pc.store.remove(p)
	logger.V(4).Info("policy is removed from cache", "name", p.GetName())
}

func (pc *policyCache) update(oldP kyverno.PolicyInterface, newP kyverno.PolicyInterface) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	pc.store.update(oldP, newP)
	logger.V(4).Info("policy is updated from cache", "name", newP.GetName())
}

func (pc *policyCache) get(pkey PolicyType, kind, nspace string) []string {
	pc.lock.RLock()
	defer pc.lock.RUnlock()
	return pc.store.get(pkey, kind, nspace)
}

// policyMap stores names of ClusterPolicies and  Namespaced Policies.
// Since both the policy name use same type (i.e. string), Both policies can be differentiated based on
// "namespace". namespace policy get stored with policy namespace with policy name"
// kindDataMap {"kind": {{"policytype" : {"policyName","nsname/policyName}}},"kind2": {{"policytype" : {"nsname/policyName" }}}}
type policyMap map[string]map[PolicyType]sets.String

func getKind(gvk string) string {
	_, k := kubeutils.GetKindFromGVK(gvk)
	kind, _ := kubeutils.SplitSubresource(k)
	return kind
}

func (m policyMap) add(policy kyverno.PolicyInterface) {
	m.addPolicyToCache(policy)
}

func (m policyMap) remove(policy kyverno.PolicyInterface) {
	m.removePolicyFromCache(policy)
}

func (m policyMap) update(old kyverno.PolicyInterface, new kyverno.PolicyInterface) {
	m.removePolicyFromCache(old)
	m.addPolicyToCache(new)
}

func (m policyMap) get(key PolicyType, gvk, namespace string) (names []string) {
	kind := getKind(gvk)
	for policyName := range m[kind][key] {
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

func (m policyMap) addPolicyToCache(policy kyverno.PolicyInterface) {
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

func (m policyMap) removePolicyFromCache(policy kyverno.PolicyInterface) {
	var pName = policy.GetName()
	pSpace := policy.GetNamespace()
	if pSpace != "" {
		pName = pSpace + "/" + pName
	}
	for _, rule := range autogen.ComputeRules(policy) {
		removeCacheHelper(rule.MatchResources, m, pName)
	}
}

func addCacheHelper(match kyverno.MatchResources, m policyMap, rule kyverno.Rule, pName string, enforcePolicy bool) {
	for _, gvk := range match.GetKinds() {
		kind := getKind(gvk)
		if m[kind] == nil {
			m[kind] = map[PolicyType]sets.String{
				Mutate:               sets.NewString(),
				ValidateEnforce:      sets.NewString(),
				ValidateAudit:        sets.NewString(),
				Generate:             sets.NewString(),
				VerifyImagesMutate:   sets.NewString(),
				VerifyImagesValidate: sets.NewString(),
			}
		}
		if rule.HasMutate() {
			m[kind][Mutate] = m[kind][Mutate].Insert(pName)
			continue
		}
		if rule.HasValidate() {
			if enforcePolicy {
				m[kind][ValidateEnforce] = m[kind][ValidateEnforce].Insert(pName)
			} else {
				m[kind][ValidateAudit] = m[kind][ValidateAudit].Insert(pName)
			}
			continue
		}
		if rule.HasGenerate() {
			m[kind][Generate] = m[kind][Generate].Insert(pName)
			continue
		}
		if rule.HasVerifyImages() {
			m[kind][VerifyImagesMutate] = m[kind][VerifyImagesMutate].Insert(pName)
			if rule.HasImagesValidationChecks() {
				m[kind][VerifyImagesValidate] = m[kind][VerifyImagesValidate].Insert(pName)
			}
			continue
		}
	}
}

func removeCacheHelper(match kyverno.MatchResources, m policyMap, pName string) {
	for _, gvk := range match.GetKinds() {
		kind := getKind(gvk)
		for policyType, policies := range m[kind] {
			m[kind][policyType] = policies.Delete(pName)
		}
	}
}
