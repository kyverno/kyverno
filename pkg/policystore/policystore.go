package policystore

import (
	"sync"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

type PolicyElement struct {
	Name string
	Rule string
}

//Operation defines the operation that a rule is performing
// we can only have a single operation per rule
type Operation string

const (
	//Mutation : mutation rules
	Mutation Operation = "Mutation"
	//Validation : validation rules
	Validation Operation = "Validation"
	//Generation : generation rules
	Generation Operation = "Generation"
)

type policyMap map[PolicyElement]interface{}

//PolicyStore Store the meta-data information to faster lookup policies
type PolicyStore struct {
	data map[Operation]map[string]map[string]policyMap
	mu   sync.RWMutex
}

type Interface interface {
	// Register a new policy
	Register(policy kyverno.ClusterPolicy)
	// Remove policy information
	UnRegister(policy kyverno.ClusterPolicy) error
	// Lookup based on kind and namespaces
	LookUp(operation Operation, kind, namespace string) []PolicyElement
}

// NewPolicyStore returns a new policy store
func NewPolicyStore() *PolicyStore {
	ps := PolicyStore{
		data: make(map[Operation]map[string]map[string]policyMap),
	}
	return &ps
}

func operation(rule kyverno.Rule) Operation {
	if rule.HasMutate() {
		return Mutation
	} else if rule.HasValidate() {
		return Validation
	} else {
		return Generation
	}
}

//Register a new policy
func (ps *PolicyStore) Register(policy kyverno.ClusterPolicy) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var pmap policyMap
	// add an entry for each rule in policy
	for _, rule := range policy.Spec.Rules {
		// get operation
		operation := operation(rule)
		operationMap := ps.addOperation(operation)

		//		rule.MatchResources.Kinds - List - mandatory - atleast on entry
		for _, kind := range rule.MatchResources.Kinds {
			kindMap := addKind(operationMap, kind)
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
			addPolicyElement(pmap, policy.Name, rule.Name)
		}
	}
}

//UnRegister Remove policy information
func (ps *PolicyStore) UnRegister(policy kyverno.ClusterPolicy) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, rule := range policy.Spec.Rules {
		// get operation
		operation := operation(rule)
		operationMap := ps.getOperation(operation)
		for _, kind := range rule.MatchResources.Kinds {
			// get kind Map
			kindMap := getKind(operationMap, kind)
			if kindMap == nil {
				// kind does not exist
				return nil
			}
			if len(rule.MatchResources.Namespaces) == 0 {
				namespace := "*"
				pmap := getNamespace(kindMap, namespace)
				// remove element
				delete(pmap, PolicyElement{Name: policy.Name, Rule: rule.Name})
			} else {
				for _, ns := range rule.MatchResources.Namespaces {
					pmap := getNamespace(kindMap, ns)
					// remove element
					delete(pmap, PolicyElement{Name: policy.Name, Rule: rule.Name})
				}
			}
		}
	}
	return nil
}

//LookUp lookups up the policies for kind and namespace
// returns a list of <policy, rule> that statisfy the filters
func (ps *PolicyStore) LookUp(operation Operation, kind, namespace string) []PolicyElement {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var policyMap policyMap
	var ret []PolicyElement
	// operation
	operationMap := ps.getOperation(operation)
	if operationMap == nil {
		return []PolicyElement{}
	}
	// kind
	kindMap := getKind(operationMap, kind)
	if kindMap == nil {
		return []PolicyElement{}
	}
	// get namespace specific policies
	policyMap = kindMap[namespace]
	ret = append(ret, transform(policyMap)...)
	// get policies on all namespaces
	policyMap = kindMap["*"]
	ret = append(ret, transform(policyMap)...)
	return ret
}

// generates a copy
func transform(pmap policyMap) []PolicyElement {
	ret := []PolicyElement{}
	for k := range pmap {
		ret = append(ret, k)
	}
	return ret
}

func (ps *PolicyStore) addOperation(operation Operation) map[string]map[string]policyMap {
	operationMap, ok := ps.data[operation]
	if ok {
		return operationMap
	}
	ps.data[operation] = make(map[string]map[string]policyMap)
	return ps.data[operation]
}

func (ps *PolicyStore) getOperation(operation Operation) map[string]map[string]policyMap {
	return ps.data[operation]
}

func addKind(operationMap map[string]map[string]policyMap, kind string) map[string]policyMap {
	val, ok := operationMap[kind]
	if ok {
		return val
	}
	operationMap[kind] = make(map[string]policyMap)
	return operationMap[kind]
}

func getKind(operationMap map[string]map[string]policyMap, kind string) map[string]policyMap {
	return operationMap[kind]
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

func addPolicyElement(pmap policyMap, name, rule string) {
	var emptyInterface interface{}
	key := PolicyElement{
		Name: name,
		Rule: rule,
	}
	if _, ok := pmap[key]; !ok {
		pmap[key] = emptyInterface
	}
}
