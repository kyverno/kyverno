package policystore

import (
	"encoding/json"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

func Test_Add(t *testing.T) {
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
	var policy1 kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy1, &policy1)
	var policy2 kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy2, &policy2)

	var store Interface
	store = NewPolicyStore()
	// Add
	store.Register(policy1)
	store.Register(policy2)
	t.Log(store.LookUp(Mutation, "Pod", ""))
	store.UnRegister(policy1)
	t.Log(store.LookUp(Mutation, "Pod", ""))
	store.Register(policy1)
	t.Log(store.LookUp(Mutation, "Pod", ""))
	t.Fail()
}
