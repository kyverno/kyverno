package policystore

import (
	"sync"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
)

type policyMap map[string]interface{}
type namespaceMap map[string]policyMap
type kindMap map[string]namespaceMap

//PolicyStore Store the meta-data information to faster lookup policies
type PolicyStore struct {
	data    map[string]namespaceMap
	mu      sync.RWMutex
	pLister kyvernolister.ClusterPolicyLister
}

//UpdateInterface provides api to update policies
type UpdateInterface interface {
	// Register a new policy
	Register(policy kyverno.ClusterPolicy)
	// Remove policy information
	UnRegister(policy kyverno.ClusterPolicy) error
}

//LookupInterface provides api to lookup policies
type LookupInterface interface {
	// Lookup based on kind and namespaces
	LookUp(kind, namespace string) ([]kyverno.ClusterPolicy, error)
}

// NewPolicyStore returns a new policy store
func NewPolicyStore(pLister kyvernolister.ClusterPolicyLister) *PolicyStore {
	ps := PolicyStore{
		data:    make(kindMap),
		pLister: pLister,
	}
	return &ps
}

//Register a new policy
func (ps *PolicyStore) Register(policy kyverno.ClusterPolicy) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var pmap policyMap
	// add an entry for each rule in policy
	for _, rule := range policy.Spec.Rules {
		//		rule.MatchResources.Kinds - List - mandatory - atleast on entry
		for _, kind := range rule.MatchResources.Kinds {
			kindMap := ps.addKind(kind)
			// namespaces
			if len(rule.MatchResources.Namespaces) == 0 {
				// all namespaces - *
				pmap = addNamespace(kindMap, "*")
			} else {
				for _, ns := range rule.MatchResources.Namespaces {
					pmap = addNamespace(kindMap, ns)
				}
			}
			// add policy to the pmap
			addPolicyElement(pmap, policy.Name)
		}
	}
}

//LookUp look up the resources
func (ps *PolicyStore) LookUp(kind, namespace string) ([]kyverno.ClusterPolicy, error) {
	ret := []kyverno.ClusterPolicy{}
	// lookup meta-store
	policyNames := ps.lookUp(kind, namespace)
	for _, policyName := range policyNames {
		policy, err := ps.pLister.Get(policyName)
		if err != nil {
			return nil, err
		}
		ret = append(ret, *policy)
	}
	return ret, nil
}

//UnRegister Remove policy information
func (ps *PolicyStore) UnRegister(policy kyverno.ClusterPolicy) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {
			// get kind Map
			kindMap := ps.getKind(kind)
			if kindMap == nil {
				// kind does not exist
				return nil
			}
			if len(rule.MatchResources.Namespaces) == 0 {
				namespace := "*"
				pmap := getNamespace(kindMap, namespace)
				// remove element
				delete(pmap, policy.Name)
			} else {
				for _, ns := range rule.MatchResources.Namespaces {
					pmap := getNamespace(kindMap, ns)
					// remove element
					delete(pmap, policy.Name)
				}
			}
		}
	}
	return nil
}

//LookUp lookups up the policies for kind and namespace
// returns a list of <policy, rule> that statisfy the filters
func (ps *PolicyStore) lookUp(kind, namespace string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var policyMap policyMap
	var ret []string
	// kind
	kindMap := ps.getKind(kind)
	if kindMap == nil {
		return []string{}
	}
	// get namespace specific policies
	policyMap = kindMap[namespace]
	ret = append(ret, transform(policyMap)...)
	// get policies on all namespaces
	policyMap = kindMap["*"]
	ret = append(ret, transform(policyMap)...)
	return unique(ret)
}

func unique(intSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// generates a copy
func transform(pmap policyMap) []string {
	ret := []string{}
	for k := range pmap {
		ret = append(ret, k)
	}
	return ret
}

func (ps *PolicyStore) addKind(kind string) namespaceMap {
	val, ok := ps.data[kind]
	if ok {
		return val
	}
	ps.data[kind] = make(namespaceMap)
	return ps.data[kind]
}

func (ps *PolicyStore) getKind(kind string) namespaceMap {
	return ps.data[kind]
}

func addNamespace(kindMap map[string]policyMap, namespace string) policyMap {
	val, ok := kindMap[namespace]
	if ok {
		return val
	}
	kindMap[namespace] = make(policyMap)
	return kindMap[namespace]
}

func getNamespace(kindMap map[string]policyMap, namespace string) policyMap {
	return kindMap[namespace]
}

func addPolicyElement(pmap policyMap, name string) {
	var emptyInterface interface{}

	if _, ok := pmap[name]; !ok {
		pmap[name] = emptyInterface
	}
}
