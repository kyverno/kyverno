package policycache

import (
	"strings"
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kcache "k8s.io/client-go/tools/cache"
)

type store interface {
	// set inserts a policy in the cache
	set(string, kyvernov1.PolicyInterface, ResourceFinder) error
	// unset removes a policy from the cache
	unset(string)
	// get finds policies that match a given type, gvr and namespace
	get(PolicyType, schema.GroupVersionResource, string) []kyvernov1.PolicyInterface
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

func (pc *policyCache) set(key string, policy kyvernov1.PolicyInterface, client ResourceFinder) error {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	if err := pc.store.set(key, policy, client); err != nil {
		return err
	}
	logger.V(4).Info("policy is added to cache", "key", key)
	return nil
}

func (pc *policyCache) unset(key string) {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	pc.store.unset(key)
	logger.V(4).Info("policy is removed from cache", "key", key)
}

func (pc *policyCache) get(pkey PolicyType, gvr schema.GroupVersionResource, nspace string) []kyvernov1.PolicyInterface {
	pc.lock.RLock()
	defer pc.lock.RUnlock()
	return pc.store.get(pkey, gvr, nspace)
}

type policyMap struct {
	// policies maps names to policy interfaces
	policies map[string]kyvernov1.PolicyInterface
	// kindType stores names of policies ClusterPolicies and Namespaced Policies.
	// They are accessed first by GVR then by PolicyType.
	kindType map[schema.GroupVersionResource]map[PolicyType]sets.Set[string]
}

func newPolicyMap() *policyMap {
	return &policyMap{
		policies: map[string]kyvernov1.PolicyInterface{},
		kindType: map[schema.GroupVersionResource]map[PolicyType]sets.Set[string]{},
	}
}

func computeEnforcePolicy(spec *kyvernov1.Spec) bool {
	if spec.ValidationFailureAction.Enforce() {
		return true
	}
	for _, k := range spec.ValidationFailureActionOverrides {
		if k.Action.Enforce() {
			return true
		}
	}
	return false
}

func set(set sets.Set[string], item string, value bool) sets.Set[string] {
	if value {
		return set.Insert(item)
	} else {
		return set.Delete(item)
	}
}

func (m *policyMap) set(key string, policy kyvernov1.PolicyInterface, client ResourceFinder) error {
	enforcePolicy := computeEnforcePolicy(policy.GetSpec())
	m.policies[key] = policy
	type state struct {
		hasMutate, hasValidate, hasGenerate, hasVerifyImages, hasImagesValidationChecks, hasVerifyYAML bool
	}
	kindStates := map[schema.GroupVersionResource]state{}
	for _, rule := range autogen.ComputeRules(policy) {
		for _, gvk := range rule.MatchResources.GetKinds() {
			group, version, kind, subresource := kubeutils.ParseKindSelector(gvk)
			gvrs, err := client.FindResources(group, version, kind, subresource)
			if err != nil {
				logger.Error(err, "failed to fetch resource group versions", "group", group, "version", version, "kind", kind)
				// TODO: keep processing or return ?
				return err
			}
			// TODO: account for pods/ephemeralcontainers
			for _, gvr := range gvrs {
				gvr.Resource = strings.Split(gvr.Resource, "/")[0]
				entry := kindStates[gvr]
				entry.hasMutate = entry.hasMutate || rule.HasMutate()
				entry.hasValidate = entry.hasValidate || rule.HasValidate()
				entry.hasGenerate = entry.hasGenerate || rule.HasGenerate()
				entry.hasVerifyImages = entry.hasVerifyImages || rule.HasVerifyImages()
				entry.hasImagesValidationChecks = entry.hasImagesValidationChecks || rule.HasImagesValidationChecks()
				// TODO: hasVerifyYAML
				kindStates[gvr] = entry
			}
		}
	}
	for gvr, state := range kindStates {
		if m.kindType[gvr] == nil {
			m.kindType[gvr] = map[PolicyType]sets.Set[string]{
				Mutate:               sets.New[string](),
				ValidateEnforce:      sets.New[string](),
				ValidateAudit:        sets.New[string](),
				Generate:             sets.New[string](),
				VerifyImagesMutate:   sets.New[string](),
				VerifyImagesValidate: sets.New[string](),
				VerifyYAML:           sets.New[string](),
			}
		}
		m.kindType[gvr][Mutate] = set(m.kindType[gvr][Mutate], key, state.hasMutate)
		m.kindType[gvr][ValidateEnforce] = set(m.kindType[gvr][ValidateEnforce], key, state.hasValidate && enforcePolicy)
		m.kindType[gvr][ValidateAudit] = set(m.kindType[gvr][ValidateAudit], key, state.hasValidate && !enforcePolicy)
		m.kindType[gvr][Generate] = set(m.kindType[gvr][Generate], key, state.hasGenerate)
		m.kindType[gvr][VerifyImagesMutate] = set(m.kindType[gvr][VerifyImagesMutate], key, state.hasVerifyImages)
		m.kindType[gvr][VerifyImagesValidate] = set(m.kindType[gvr][VerifyImagesValidate], key, state.hasVerifyImages && state.hasImagesValidationChecks)
		m.kindType[gvr][VerifyYAML] = set(m.kindType[gvr][VerifyYAML], key, state.hasVerifyYAML)
	}
	return nil
}

func (m *policyMap) unset(key string) {
	delete(m.policies, key)
	for gvr := range m.kindType {
		for policyType := range m.kindType[gvr] {
			m.kindType[gvr][policyType] = m.kindType[gvr][policyType].Delete(key)
		}
	}
}

func (m *policyMap) get(key PolicyType, gvr schema.GroupVersionResource, namespace string) []kyvernov1.PolicyInterface {
	var result []kyvernov1.PolicyInterface
	for policyName := range m.kindType[gvr][key] {
		ns, _, err := kcache.SplitMetaNamespaceKey(policyName)
		if err != nil {
			logger.Error(err, "failed to parse policy name", "policyName", policyName)
		}
		isNamespacedPolicy := ns != ""
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
