package policycache

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"

	lv1 "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type dummyLister struct {
}

func (dl dummyLister) List(selector labels.Selector) (ret []*kyverno.ClusterPolicy, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (dl dummyLister) Get(name string) (*kyverno.ClusterPolicy, error) {
	return nil, fmt.Errorf("not implemented")
}

func (dl dummyLister) ListResources(selector labels.Selector) (ret []*kyverno.ClusterPolicy, err error) {
	return nil, fmt.Errorf("not implemented")
}

// type dymmyNsNamespace struct {}

type dummyNsLister struct {
}

func (dl dummyNsLister) Policies(name string) lv1.PolicyNamespaceLister {
	return dummyNsLister{}
}

func (dl dummyNsLister) List(selector labels.Selector) (ret []*kyverno.Policy, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (dl dummyNsLister) Get(name string) (*kyverno.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

func Test_All(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newPolicy(t)
	//add
	pCache.Add(policy)
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {

			// get
			mutate := pCache.Get(Mutate, &kind, nil)
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.Get(ValidateEnforce, &kind, nil)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.Get(Generate, &kind, nil)
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}

	// remove
	pCache.Remove(policy)
	kind := "pod"
	validateEnforce := pCache.Get(ValidateEnforce, &kind, nil)
	assert.Assert(t, len(validateEnforce) == 0)
}

func Test_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newPolicy(t)
	pCache.Add(policy)
	pCache.Add(policy)
	pCache.Add(policy)
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {

			mutate := pCache.Get(Mutate, &kind, nil)
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.Get(ValidateEnforce, &kind, nil)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.Get(Generate, &kind, nil)
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}
}

func Test_Add_Validate_Audit(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newPolicy(t)
	pCache.Add(policy)
	pCache.Add(policy)

	policy.Spec.ValidationFailureAction = "audit"
	pCache.Add(policy)
	pCache.Add(policy)
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {

			validateEnforce := pCache.Get(ValidateEnforce, &kind, nil)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(validateEnforce))
			}

			validateAudit := pCache.Get(ValidateAudit, &kind, nil)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateAudit))
			}
		}
	}
}

func Test_Add_Remove(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newPolicy(t)
	kind := "Pod"
	pCache.Add(policy)
	validateEnforce := pCache.Get(ValidateEnforce, &kind, nil)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	pCache.Remove(policy)
	deletedValidateEnforce := pCache.Get(ValidateEnforce, &kind, nil)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_Remove_From_Empty_Cache(t *testing.T) {
	pCache := newPolicyCache(log.Log, nil, nil)
	policy := newPolicy(t)

	pCache.Remove(policy)
}

func newPolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"metadata": {
		  "name": "test-policy"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "rules": [
			{
			  "name": "deny-privileged-disallowpriviligedescalation",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod",
					"Namespace"
				  ]
				}
			  },
			  "validate": {
				"deny": {
				  "conditions": {
					"all": [
					  {
						"key": "a",
						"operator": "Equals",
						"value": "a"
					  }
					]
				  }
				}
			  }
			},
			{
			  "name": "deny-privileged-disallowpriviligedescalation",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"containers": [
					  {
						"image": "!*:latest"
					  }
					]
				  }
				}
			  }
			},
			{
			  "name": "annotate-host-path",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod",
					"Namespace"
				  ]
				}
			  },
			  "mutate": {
				"overlay": {
				  "metadata": {
					"annotations": {
					  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": true
					}
				  }
				}
			  }
			},
			{
			  "name": "default-deny-ingress",
			  "match": {
				"resources": {
				  "kinds": [
					"Namespace",
					"Pod"
				  ]
				}
			  },
			  "generate": {
				"kind": "NetworkPolicy",
				"name": "default-deny-ingress",
				"namespace": "{{request.object.metadata.name}}",
				"data": {
				  "spec": {
					"podSelector": {
					},
					"policyTypes": [
					  "Ingress"
					]
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	return policy
}

func newNsPolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"metadata": {
		  "name": "test-policy",
		  "namespace": "test"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "rules": [
			{
			  "name": "deny-privileged-disallowpriviligedescalation",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"deny": {
				  "conditions": {
					"all": [
						{
							"key": "a",
							"operator": "Equals",
							"value": "a"
						}
					]
				  } 
				}
			  }
			},
			{
			  "name": "deny-privileged-disallowpriviligedescalation",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"containers": [
					  {
						"image": "!*:latest"
					  }
					]
				  }
				}
			  }
			},
			{
			  "name": "annotate-host-path",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"overlay": {
				  "metadata": {
					"annotations": {
					  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": true
					}
				  }
				}
			  }
			},
			{
			  "name": "default-deny-ingress",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "generate": {
				"kind": "NetworkPolicy",
				"name": "default-deny-ingress",
				"namespace": "{{request.object.metadata.name}}",
				"data": {
				  "spec": {
					"podSelector": {
					},
					"policyTypes": [
					  "Ingress"
					]
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	var policy *kyverno.Policy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	return convertPolicyToClusterPolicy(policy)
}

func Test_Ns_All(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newNsPolicy(t)
	//add
	pCache.Add(policy)
	nspace := policy.GetNamespace()
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {

			// get
			mutate := pCache.Get(Mutate, &kind, &nspace)
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.Get(ValidateEnforce, &kind, &nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.Get(Generate, &kind, &nspace)
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}
	// remove
	pCache.Remove(policy)
	kind := "pod"
	validateEnforce := pCache.Get(ValidateEnforce, &kind, &nspace)
	assert.Assert(t, len(validateEnforce) == 0)
}

func Test_Ns_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newNsPolicy(t)
	pCache.Add(policy)
	pCache.Add(policy)
	pCache.Add(policy)
	nspace := policy.GetNamespace()
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {

			mutate := pCache.Get(Mutate, &kind, &nspace)
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.Get(ValidateEnforce, &kind, &nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.Get(Generate, &kind, &nspace)
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}
}

func Test_Ns_Add_Validate_Audit(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newNsPolicy(t)
	pCache.Add(policy)
	pCache.Add(policy)
	nspace := policy.GetNamespace()
	policy.Spec.ValidationFailureAction = "audit"
	pCache.Add(policy)
	pCache.Add(policy)
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {

			validateEnforce := pCache.Get(ValidateEnforce, &kind, &nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}

			validateAudit := pCache.Get(ValidateAudit, &kind, &nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateAudit))
			}
		}
	}
}

func Test_Ns_Add_Remove(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newNsPolicy(t)
	nspace := policy.GetNamespace()
	kind := "Pod"
	pCache.Add(policy)
	validateEnforce := pCache.Get(ValidateEnforce, &kind, &nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	pCache.Remove(policy)
	deletedValidateEnforce := pCache.Get(ValidateEnforce, &kind, &nspace)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}
