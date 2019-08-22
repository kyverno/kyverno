package policystore

import (
	"sync"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Interface interface {
	Register(policy *kyverno.Policy) error
	UnRegister(policy *kyverno.Policy) error                       // check if the controller can see the policy spec for details?
	LookUp(kind, namespace, name string, ls *metav1.LabelSelector) // returns a list of policies and rules that apply
}

type Store struct {
	data map[string]string
	mux  sync.RWMutex
}

func NewStore() *Store {
	s := Store{
		data: make(map[string]string), //key: kind, value is the name of the policy
	}

	return &s
}

var empty struct{}

func (s *Store) Register(policy *kyverno.Policy) error {
	// check if this policy is already registered for this resource kind
	kinds := map[string]string{}
	// get kinds from the rules
	for _, r := range policy.Spec.Rules {
		rkinds := map[string]string{}
		// matching resources
		for _, k := range r.MatchResources.Kinds {
			rkinds[k] = policy.Name
		}
		for _, k := range r.ExcludeResources.Kinds {
			delete(rkinds, k)
		}
		// merge the result
		mergeMap(kinds, rkinds)

	}

	// have all the kinds that the policy has rule on
	s.mux.Lock()
	defer s.mux.Unlock()
	// merge kinds
	mergeMap(s.data, kinds)

	return nil
}

// merge m2 into m2
func mergeMap(m1, m2 map[string]string) {
	for k, v := range m2 {
		m1[k] = v
	}
}
