package policystore

import (
	"encoding/json"
	"reflect"
	"testing"

	v1alpha1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/fake"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func Test_Operations(t *testing.T) {
	rawPolicy1 := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test-policy"
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
		"apiVersion": "kyverno.io/v1alpha1",
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
		"apiVersion": "kyverno.io/v1alpha1",
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
	var policy1 v1alpha1.ClusterPolicy
	json.Unmarshal(rawPolicy1, &policy1)
	var policy2 v1alpha1.ClusterPolicy
	json.Unmarshal(rawPolicy2, &policy2)
	var policy3 v1alpha1.ClusterPolicy
	json.Unmarshal(rawPolicy3, &policy3)
	scheme.Scheme.AddKnownTypes(v1alpha1.SchemeGroupVersion,
		&v1alpha1.ClusterPolicy{},
	)
	var obj runtime.Object
	var err error
	var retPolicies []v1alpha1.ClusterPolicy
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
	fakeLister := &FakeLister{client: client}
	store := NewPolicyStore(fakeLister)
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
	if !reflect.DeepEqual(retPolicies, []v1alpha1.ClusterPolicy{policy1, policy2}) {
		t.Error("not matching")
	}

	// Remove
	store.UnRegister(policy1)
	retPolicies, err = store.LookUp("Pod", "")
	if err != nil {
		t.Error(err)
	}
	// Lookup
	if !reflect.DeepEqual(retPolicies, []v1alpha1.ClusterPolicy{policy2}) {
		t.Error("not matching")
	}
	// Add
	store.Register(policy1)
	retPolicies, err = store.LookUp("Pod", "")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(retPolicies, []v1alpha1.ClusterPolicy{policy1, policy2}) {
		t.Error("not matching")
	}

	retPolicies, err = store.LookUp("Service", "")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(retPolicies, []v1alpha1.ClusterPolicy{policy3}) {
		t.Error("not matching")
	}

}

type FakeLister struct {
	client *fake.Clientset
}

func (fk *FakeLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterPolicy, err error) {
	return nil, nil
}

func (fk *FakeLister) Get(name string) (*v1alpha1.ClusterPolicy, error) {
	return fk.client.KyvernoV1alpha1().ClusterPolicies().Get(name, v1.GetOptions{})
}

func (fk *FakeLister) GetPolicyForPolicyViolation(pv *v1alpha1.ClusterPolicyViolation) ([]*v1alpha1.ClusterPolicy, error) {
	return nil, nil
}
func (fk *FakeLister) ListResources(selector labels.Selector) (ret []*v1alpha1.ClusterPolicy, err error) {
	return nil, nil
}

func (fk *FakeLister) GetPolicyForNamespacedPolicyViolation(pv *v1alpha1.NamespacedPolicyViolation) ([]*v1alpha1.ClusterPolicy, error) {
	return nil, nil
}
