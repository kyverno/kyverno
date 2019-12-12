package policystore

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/fake"
	listerv1 "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	cache "k8s.io/client-go/tools/cache"
)

func Test_Operations(t *testing.T) {
	rawPolicy1 := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test-policy1"
		},
		"spec": {
		  "rules": [
			{
			  "name": "r1",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"overlay": "temp"
			  }
			},
			{
			  "name": "r2",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod",
					"Deployment"
				  ]
				}
			  },
			  "mutate": {
				"overlay": "temp"
			  }
			},
			{
			  "name": "r3",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod",
					"Deployment"
				  ],
				  "namespaces": [
					"n1"
				  ]
				}
			  },
			  "mutate": {
				"overlay": "temp"
			  }
			},
			{
			  "name": "r4",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod",
					"Deployment"
				  ],
				  "namespaces": [
					"n1",
					"n2"
				  ]
				}
			  },
			  "validate": {
				"pattern": "temp"
			  }
			}
		  ]
		}
	  }
	`)

	rawPolicy2 := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test-policy2"
		},
		"spec": {
		  "rules": [
			{
			  "name": "r1",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"overlay": "temp"
			  }
			},
			{
			  "name": "r2",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ],
				  "namespaces": [
					"n4"
				  ]
				}
			  },
			  "mutate": {
				"overlay": "temp"
			  }
			},
			{
			  "name": "r2",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ],
				  "namespaces": [
					"n4",
					"n5",
					"n6"
				  ]
				}
			  },
			  "validate": {
				"pattern": "temp"
			  }
			}
		  ]
		}
	  }`)

	rawPolicy3 := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test-policy3"
		},
		"spec": {
		  "rules": [
			{
			  "name": "r4",
			  "match": {
				"resources": {
				  "kinds": [
					"Service"
				  ]
				}
			  },
			  "mutate": {
				"overlay": "temp"
			  }
			}
		  ]
		}
	  }`)
	var policy1 kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy1, &policy1)
	var policy2 kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy2, &policy2)
	var policy3 kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy3, &policy3)
	scheme.Scheme.AddKnownTypes(kyverno.SchemeGroupVersion,
		&kyverno.ClusterPolicy{},
	)
	var obj runtime.Object
	var err error
	var retPolicies []kyverno.ClusterPolicy
	polices := []runtime.Object{}
	// list of runtime objects
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err = decode(rawPolicy1, nil, nil)
	if err != nil {
		t.Error(err)
	}
	polices = append(polices, obj)
	obj, _, err = decode(rawPolicy2, nil, nil)
	if err != nil {
		t.Error(err)
	}
	polices = append(polices, obj)
	obj, _, err = decode(rawPolicy3, nil, nil)
	if err != nil {
		t.Error(err)
	}
	polices = append(polices, obj)
	// Mock Lister
	client := fake.NewSimpleClientset(polices...)
	fakeInformer := &FakeInformer{client: client}
	store := NewPolicyStore(fakeInformer)
	// Test Operations
	// Add
	store.Register(policy1)
	// Add
	store.Register(policy2)
	// Add
	store.Register(policy3)
	// Lookup
	retPolicies, err = store.LookUp("Pod", "")
	if err != nil {
		t.Error(err)
	}
	if len(retPolicies) != len([]kyverno.ClusterPolicy{policy1, policy2}) {
		// checking length as the order of polcies might be different
		t.Error("not matching")
	}

	// Remove
	store.UnRegister(policy1)
	retPolicies, err = store.LookUp("Pod", "")
	if err != nil {
		t.Error(err)
	}
	// Lookup
	if !reflect.DeepEqual(retPolicies, []kyverno.ClusterPolicy{policy2}) {
		t.Error("not matching")
	}
	// Add
	store.Register(policy1)
	retPolicies, err = store.LookUp("Pod", "")
	if err != nil {
		t.Error(err)
	}

	if len(retPolicies) != len([]kyverno.ClusterPolicy{policy1, policy2}) {
		// checking length as the order of polcies might be different
		t.Error("not matching")
	}

	retPolicies, err = store.LookUp("Service", "")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(retPolicies, []kyverno.ClusterPolicy{policy3}) {
		t.Error("not matching")
	}

}

type FakeInformer struct {
	client *fake.Clientset
}

func (fi *FakeInformer) Informer() cache.SharedIndexInformer {
	fsi := &FakeSharedInformer{}
	return fsi
}

func (fi *FakeInformer) Lister() listerv1.ClusterPolicyLister {
	fl := &FakeLister{client: fi.client}
	return fl
}

type FakeLister struct {
	client *fake.Clientset
}

func (fl *FakeLister) List(selector labels.Selector) (ret []*kyverno.ClusterPolicy, err error) {
	return nil, nil
}

func (fl *FakeLister) Get(name string) (*kyverno.ClusterPolicy, error) {
	return fl.client.KyvernoV1().ClusterPolicies().Get(name, v1.GetOptions{})
}

func (fl *FakeLister) GetPolicyForPolicyViolation(pv *kyverno.ClusterPolicyViolation) ([]*kyverno.ClusterPolicy, error) {
	return nil, nil
}
func (fl *FakeLister) ListResources(selector labels.Selector) (ret []*kyverno.ClusterPolicy, err error) {
	return nil, nil
}

func (fl *FakeLister) GetPolicyForNamespacedPolicyViolation(pv *kyverno.PolicyViolation) ([]*kyverno.ClusterPolicy, error) {
	return nil, nil
}

type FakeSharedInformer struct {
}

func (fsi *FakeSharedInformer) HasSynced() bool {
	return true
}

func (fsi *FakeSharedInformer) AddEventHandler(handler cache.ResourceEventHandler) {
}

func (fsi *FakeSharedInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {

}

func (fsi *FakeSharedInformer) AddIndexers(indexers cache.Indexers) error {
	return nil
}

func (fsi *FakeSharedInformer) GetIndexer() cache.Indexer {
	return nil
}

func (fsi *FakeSharedInformer) GetStore() cache.Store {
	return nil
}

func (fsi *FakeSharedInformer) GetController() cache.Controller {
	return nil
}

func (fsi *FakeSharedInformer) Run(stopCh <-chan struct{}) {

}

func (fsi *FakeSharedInformer) LastSyncResourceVersion() string {
	return ""
}
