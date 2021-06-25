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
			mutate := pCache.get(Mutate, kind, "")
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.get(ValidateEnforce, kind, "")
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.get(Generate, kind, "")
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}

	// remove
	pCache.Remove(policy)
	kind := "pod"
	validateEnforce := pCache.get(ValidateEnforce, kind, "")
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

			mutate := pCache.get(Mutate, kind, "")
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.get(ValidateEnforce, kind, "")
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.get(Generate, kind, "")
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

			validateEnforce := pCache.get(ValidateEnforce, kind, "")
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}

			validateAudit := pCache.get(ValidateAudit, kind, "")
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
	validateEnforce := pCache.get(ValidateEnforce, kind, "")
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	pCache.Remove(policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, kind, "")
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

func newGVKPolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"metadata": {
		   "name": "add-networkpolicy1",
		   "annotations": {
			  "policies.kyverno.io/category": "Workload Management"
		   }
		},
		"spec": {
		   "validationFailureAction": "enforce",
		   "rules": [
			  {
				 "name": "default-deny-ingress",
				 "match": {
					"resources": {
					   "kinds": [
						  "rbac.authorization.k8s.io/v1beta1/ClusterRole"
					   ],
					   "name": "*"
					}
				 },
				 "exclude": {
					"resources": {
					   "namespaces": [
						  "kube-system",
						  "default",
						  "kube-public",
						  "kyverno"
					   ]
					}
				 },
				 "generate": {
					"kind": "NetworkPolicy",
					"name": "default-deny-ingress",
					"namespace": "default",
					"synchronize": true,
					"data": {
					   "spec": {
						  "podSelector": {},
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

func newUserTestPolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "Policy",
		"metadata": {
		   "name": "require-dep-purpose-label",
		   "namespace": "default"
		},
		"spec": {
		   "validationFailureAction": "enforce",
		   "rules": [
			  {
				 "name": "require-dep-purpose-label",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ]
					}
				 },
				 "validate": {
					"message": "You must have label purpose with value production set on all new Deployment.",
					"pattern": {
					   "metadata": {
						  "labels": {
							 "purpose": "production"
						  }
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
			mutate := pCache.get(Mutate, kind, nspace)
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.get(Generate, kind, nspace)
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}
	// remove
	pCache.Remove(policy)
	kind := "pod"
	validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
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

			mutate := pCache.get(Mutate, kind, nspace)
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}

			validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
			generate := pCache.get(Generate, kind, nspace)
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

			validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}

			validateAudit := pCache.get(ValidateAudit, kind, nspace)
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
	validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	pCache.Remove(policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_GVk_Cache(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newGVKPolicy(t)
	//add
	pCache.Add(policy)
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {

			generate := pCache.get(Generate, kind, "")
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}
}

func Test_GVK_Add_Remove(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newGVKPolicy(t)
	kind := "ClusterRole"
	pCache.Add(policy)
	generate := pCache.get(Generate, kind, "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}

	pCache.Remove(policy)
	deletedGenerate := pCache.get(Generate, kind, "")
	if len(deletedGenerate) != 0 {
		t.Errorf("expected 0 generate policy, found %v", len(deletedGenerate))
	}
}

func Test_Add_Validate_Enforce(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newUserTestPolicy(t)
	nspace := policy.GetNamespace()
	//add
	pCache.Add(policy)
	for _, rule := range policy.Spec.Rules {
		for _, kind := range rule.MatchResources.Kinds {
			validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
		}
	}
}

func Test_Ns_Add_Remove_User(t *testing.T) {
	pCache := newPolicyCache(log.Log, dummyLister{}, dummyNsLister{})
	policy := newUserTestPolicy(t)
	nspace := policy.GetNamespace()
	kind := "Deployment"
	pCache.Add(policy)
	validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	pCache.Remove(policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}
