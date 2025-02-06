package policycache

import (
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kcache "k8s.io/client-go/tools/cache"
)

type store interface {
	// set inserts a policy in the cache
	set(string, kyvernov1.PolicyInterface, ResourceFinder) error
	// unset removes a policy from the cache
	unset(string)
	// get finds policies that match a given type, gvr, subresource and namespace
	get(PolicyType, schema.GroupVersionResource, string, string) []kyvernov1.PolicyInterface
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

func (pc *policyCache) get(pkey PolicyType, gvr schema.GroupVersionResource, subresource string, nspace string) []kyvernov1.PolicyInterface {
	pc.lock.RLock()
	defer pc.lock.RUnlock()
	return pc.store.get(pkey, gvr, subresource, nspace)
}

type policyKey struct {
	Group       string
	Version     string
	Resource    string
	SubResource string
}

var podsKey = policyKey{"", "v1", "pods", ""}

type policyMap struct {
	// policies maps names to policy interfaces
	policies map[string]kyvernov1.PolicyInterface
	// kindType stores names of ClusterPolicies and Namespaced Policies.
	// They are accessed first by GVRS then by PolicyType.
	kindType map[policyKey]map[PolicyType]sets.Set[string]
}

func newPolicyMap() *policyMap {
	return &policyMap{
		policies: map[string]kyvernov1.PolicyInterface{},
		kindType: map[policyKey]map[PolicyType]sets.Set[string]{},
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
	var errs []error
	enforcePolicy := computeEnforcePolicy(policy.GetSpec())
	auditWarning := false
	if policy.GetSpec().EmitWarning != nil && *policy.GetSpec().EmitWarning {
		auditWarning = true
	}
	m.policies[key] = policy
	type state struct {
		hasMutate, hasValidate, hasGenerate, hasVerifyImages, hasImagesValidationChecks bool
	}
	kindStates := map[policyKey]state{}
	for _, rule := range autogen.Default.ComputeRules(policy, "") {
		if rule.HasValidate() {
			action := rule.Validation.FailureAction
			if action != nil && action.Enforce() {
				enforcePolicy = true
			}
			for _, k := range rule.Validation.FailureActionOverrides {
				if k.Action.Enforce() {
					enforcePolicy = true
				}
			}
		}
		entries := sets.New[policyKey]()
		for _, gvk := range rule.MatchResources.GetKinds() {
			group, version, kind, subresource := kubeutils.ParseKindSelector(gvk)
			gvrss, err := client.FindResources(group, version, kind, subresource)
			if err != nil {
				logger.Error(err, "failed to fetch resource group versions", "group", group, "version", version, "kind", kind)
				errs = append(errs, err)
			} else {
				for gvrs := range gvrss {
					entries.Insert(policyKey{
						Group:       gvrs.Group,
						Version:     gvrs.Version,
						Resource:    gvrs.Resource,
						SubResource: gvrs.SubResource,
					})
				}
			}
		}
		if entries.Len() > 0 {
			// account for pods/ephemeralcontainers special case
			if entries.Has(podsKey) {
				entries.Insert(policyKey{
					Group:       podsKey.Group,
					Version:     podsKey.Version,
					Resource:    podsKey.Resource,
					SubResource: "ephemeralcontainers",
				})
			}
			hasMutate := rule.HasMutate()
			hasValidate := rule.HasValidate()
			hasGenerate := rule.HasGenerate()
			hasVerifyImages := rule.HasVerifyImages()
			hasImagesValidationChecks := rule.HasVerifyImageChecks()
			for gvrs := range entries {
				entry := kindStates[gvrs]
				entry.hasMutate = entry.hasMutate || hasMutate
				entry.hasValidate = entry.hasValidate || hasValidate
				entry.hasGenerate = entry.hasGenerate || hasGenerate
				entry.hasVerifyImages = entry.hasVerifyImages || hasVerifyImages
				entry.hasImagesValidationChecks = entry.hasImagesValidationChecks || hasImagesValidationChecks
				kindStates[gvrs] = entry
			}
		}
	}
	for gvrs, state := range kindStates {
		if m.kindType[gvrs] == nil {
			m.kindType[gvrs] = map[PolicyType]sets.Set[string]{
				Mutate:               sets.New[string](),
				ValidateEnforce:      sets.New[string](),
				ValidateAudit:        sets.New[string](),
				ValidateAuditWarn:    sets.New[string](),
				Generate:             sets.New[string](),
				VerifyImagesMutate:   sets.New[string](),
				VerifyImagesValidate: sets.New[string](),
			}
		}
		m.kindType[gvrs][Mutate] = set(m.kindType[gvrs][Mutate], key, state.hasMutate)
		m.kindType[gvrs][ValidateEnforce] = set(m.kindType[gvrs][ValidateEnforce], key, state.hasValidate && enforcePolicy)
		m.kindType[gvrs][ValidateAudit] = set(m.kindType[gvrs][ValidateAudit], key, state.hasValidate && !enforcePolicy)
		m.kindType[gvrs][ValidateAuditWarn] = set(m.kindType[gvrs][ValidateAuditWarn], key, state.hasValidate && !enforcePolicy && auditWarning)
		m.kindType[gvrs][Generate] = set(m.kindType[gvrs][Generate], key, state.hasGenerate)
		m.kindType[gvrs][VerifyImagesMutate] = set(m.kindType[gvrs][VerifyImagesMutate], key, state.hasVerifyImages)
		m.kindType[gvrs][VerifyImagesValidate] = set(m.kindType[gvrs][VerifyImagesValidate], key, state.hasVerifyImages && state.hasImagesValidationChecks)
	}
	return multierr.Combine(errs...)
}

func (m *policyMap) unset(key string) {
	delete(m.policies, key)
	for gvrs := range m.kindType {
		for policyType := range m.kindType[gvrs] {
			m.kindType[gvrs][policyType] = m.kindType[gvrs][policyType].Delete(key)
		}
	}
}

func (m *policyMap) get(key PolicyType, gvr schema.GroupVersionResource, subresource string, namespace string) []kyvernov1.PolicyInterface {
	var result []kyvernov1.PolicyInterface
	pKey := policyKey{gvr.Group, gvr.Version, gvr.Resource, subresource}
	for policyName := range m.kindType[pKey][key] {
		ns, _, err := kcache.SplitMetaNamespaceKey(policyName)
		if err != nil {
			logger.Error(err, "failed to parse policy name", "policyName", policyName)
		}
		isNamespacedPolicy := ns != ""
		policy := m.policies[policyName]
		if policy == nil {
			logger.V(4).Info("nil policy in the cache, this should not happen")
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
