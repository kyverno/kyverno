package policystore

import (
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type policyMap map[string]interface{}
type namespaceMap map[string]policyMap
type kindMap map[string]namespaceMap

//PolicyStore Store the meta-data information to faster lookup policies
type PolicyStore struct {
	data map[string]namespaceMap
	mu   sync.RWMutex
	// list/get cluster policy
	pLister kyvernolister.ClusterPolicyLister
	// returns true if the cluster policy store has been synced at least once
	pSynched cache.InformerSynced
	log      logr.Logger
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
	ListAll() ([]kyverno.ClusterPolicy, error)
}

// NewPolicyStore returns a new policy store
func NewPolicyStore(pInformer kyvernoinformer.ClusterPolicyInformer,
	log logr.Logger) *PolicyStore {
	ps := PolicyStore{
		data:     make(kindMap),
		pLister:  pInformer.Lister(),
		pSynched: pInformer.Informer().HasSynced,
		log:      log,
	}
	return &ps
}

//Run checks syncing
func (ps *PolicyStore) Run(stopCh <-chan struct{}) {
	logger := ps.log
	if !cache.WaitForCacheSync(stopCh, ps.pSynched) {
		logger.Info("failed to sync informer cache")
	}
}

//Register a new policy
func (ps *PolicyStore) Register(policy kyverno.ClusterPolicy) {
	logger := ps.log
	logger.V(4).Info("adding policy", "name", policy.Name)
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

func (ps *PolicyStore) ListAll() ([]kyverno.ClusterPolicy, error) {
	policyPointers, err := ps.pLister.List(labels.NewSelector())
	if err != nil {
		return nil, err
	}

	var policies = make([]kyverno.ClusterPolicy, 0, len(policyPointers))
	for _, policy := range policyPointers {
		policies = append(policies, *policy)
	}

	return policies, nil
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
