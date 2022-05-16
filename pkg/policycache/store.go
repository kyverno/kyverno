package policycache

import (
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/policy"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/util/sets"
)

type store interface {
	// set inserts a policy in the cache
	set(kyvernov1.PolicyInterface)
	// unset removes a policy from the cache
	unset(kyvernov1.PolicyInterface)
	// get finds policies that match a given type, gvk and namespace
	get(PolicyType, string, string) []kyvernov1.PolicyInterface
}

type policyCache struct {
	store store
	lock  sync.RWMutex
}

func newPolicyCache() store {
	return &policyCache{
		store: newPolicyMap(),
	}
}

func (pc *policyCache) set(policy kyvernov1.PolicyInterface) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	pc.store.set(policy)
	logger.V(4).Info("policy is added to cache", "name", policy.GetName())
}

func (pc *policyCache) unset(p kyvernov1.PolicyInterface) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	pc.store.unset(p)
	logger.V(4).Info("policy is removed from cache", "name", p.GetName())
}

func (pc *policyCache) get(pkey PolicyType, kind, nspace string) []kyvernov1.PolicyInterface {
	pc.lock.RLock()
	defer pc.lock.RUnlock()
	return pc.store.get(pkey, kind, nspace)
}

type policyMap struct {
	// policies maps names to policy interfaces
	policies map[string]kyvernov1.PolicyInterface
	// kindType stores names of ClusterPolicies and Namespaced Policies.
	// Since both the policy name use same type (i.e. string), Both policies can be differentiated based on
	// "namespace". namespace policy get stored with policy namespace with policy name"
	// kindDataMap {"kind": {{"policytype" : {"policyName","nsname/policyName}}},"kind2": {{"policytype" : {"nsname/policyName" }}}}
	kindType map[string]map[PolicyType]sets.String
}

func newPolicyMap() *policyMap {
	return &policyMap{
		policies: map[string]kyvernov1.PolicyInterface{},
		kindType: map[string]map[PolicyType]sets.String{},
	}
}

func computeKind(gvk string) string {
	_, k := kubeutils.GetKindFromGVK(gvk)
	kind, _ := kubeutils.SplitSubresource(k)
	return kind
}

func computeKey(policy kyvernov1.PolicyInterface) string {
	namespace := policy.GetNamespace()
	if namespace == "" {
		return policy.GetName()
	} else {
		return namespace + "/" + policy.GetName()
	}
}

func computeEnforcePolicy(spec *kyvernov1.Spec) bool {
	if spec.GetValidationFailureAction() == kyvernov1.Enforce {
		return true
	}
	for _, k := range spec.ValidationFailureActionOverrides {
		if k.Action == kyvernov1.Enforce {
			return true
		}
	}
	return false
}

func set(set sets.String, item string, value bool) sets.String {
	if value {
		return set.Insert(item)
	} else {
		return set.Delete(item)
	}
}

func (m *policyMap) set(policy kyvernov1.PolicyInterface) {
	key, enforcePolicy := computeKey(policy), computeEnforcePolicy(policy.GetSpec())
	m.policies[key] = policy
	type state struct {
		hasMutate, hasValidate, hasGenerate, hasVerifyImages, hasImagesValidationChecks bool
	}
	kindStates := map[string]state{}
	for _, rule := range autogen.ComputeRules(policy) {
		for _, gvk := range rule.MatchResources.GetKinds() {
			kind := computeKind(gvk)
			entry := kindStates[kind]
			entry.hasMutate = (entry.hasMutate || rule.HasMutate())
			entry.hasValidate = (entry.hasValidate || rule.HasValidate())
			entry.hasGenerate = (entry.hasGenerate || rule.HasGenerate())
			entry.hasVerifyImages = (entry.hasVerifyImages || rule.HasVerifyImages())
			entry.hasImagesValidationChecks = (entry.hasImagesValidationChecks || rule.HasImagesValidationChecks())
			kindStates[kind] = entry
		}
	}
	for kind, state := range kindStates {
		if m.kindType[kind] == nil {
			m.kindType[kind] = map[PolicyType]sets.String{
				Mutate:               sets.NewString(),
				ValidateEnforce:      sets.NewString(),
				ValidateAudit:        sets.NewString(),
				Generate:             sets.NewString(),
				VerifyImagesMutate:   sets.NewString(),
				VerifyImagesValidate: sets.NewString(),
			}
		}
		m.kindType[kind][Mutate] = set(m.kindType[kind][Mutate], key, state.hasMutate)
		m.kindType[kind][ValidateEnforce] = set(m.kindType[kind][ValidateEnforce], key, state.hasValidate && enforcePolicy)
		m.kindType[kind][ValidateAudit] = set(m.kindType[kind][ValidateAudit], key, state.hasValidate && !enforcePolicy)
		m.kindType[kind][Generate] = set(m.kindType[kind][Generate], key, state.hasGenerate)
		m.kindType[kind][VerifyImagesMutate] = set(m.kindType[kind][VerifyImagesMutate], key, state.hasVerifyImages)
		m.kindType[kind][VerifyImagesValidate] = set(m.kindType[kind][VerifyImagesValidate], key, state.hasVerifyImages && state.hasImagesValidationChecks)
	}
}

func (m *policyMap) unset(policy kyvernov1.PolicyInterface) {
	key := computeKey(policy)
	delete(m.policies, key)
	for kind := range m.kindType {
		for policyType := range m.kindType[kind] {
			m.kindType[kind][policyType] = m.kindType[kind][policyType].Delete(key)
		}
	}
}

func (m *policyMap) get(key PolicyType, gvk, namespace string) []kyvernov1.PolicyInterface {
	kind := computeKind(gvk)
	var result []kyvernov1.PolicyInterface
	for policyName := range m.kindType[kind][key] {
		ns, _, isNamespacedPolicy := policy.ParseNamespacedPolicy(policyName)
		policy := m.policies[policyName]
		if policy == nil {
			logger.Info("nil policy in the cache, this should not happen")
		}
		if !isNamespacedPolicy && namespace == "" {
			result = append(result, policy)
		} else {
			if ns == namespace {
				result = append(result, policy)
			}
		}
	}
	return result
}
