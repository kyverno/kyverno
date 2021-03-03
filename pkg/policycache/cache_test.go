package policycache

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Test_All(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newPolicy(t)

	// add
	pCache.Add(policy)

	// get
	if len(pCache.Get(Mutate, nil)) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(pCache.Get(Mutate, nil)))
	}

	if len(pCache.Get(ValidateEnforce, nil)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, nil)))
	}

	if len(pCache.Get(Generate, nil)) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(pCache.Get(Generate, nil)))
	}

	// remove
	pCache.Remove(policy)
	assert.Assert(t, len(pCache.Get(ValidateEnforce, nil)) == 0)
}

func Test_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newPolicy(t)

	pCache.Add(policy)
	pCache.Add(policy)
	pCache.Add(policy)

	if len(pCache.Get(Mutate, nil)) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(pCache.Get(Mutate, nil)))
	}

	if len(pCache.Get(ValidateEnforce, nil)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, nil)))
	}

	if len(pCache.Get(Generate, nil)) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(pCache.Get(Generate, nil)))
	}
}

func Test_Add_Validate_Audit(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newPolicy(t)

	pCache.Add(policy)
	pCache.Add(policy)

	policy.Spec.ValidationFailureAction = "audit"
	pCache.Add(policy)
	pCache.Add(policy)

	if len(pCache.Get(ValidateEnforce, nil)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, nil)))
	}

	if len(pCache.Get(ValidateAudit, nil)) != 1 {
		t.Errorf("expected 1 validate audit policy, found %v", len(pCache.Get(ValidateAudit, nil)))
	}
}

func Test_Add_Remove(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newPolicy(t)

	pCache.Add(policy)
	if len(pCache.Get(ValidateEnforce, nil)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, nil)))
	}

	pCache.Remove(policy)
	if len(pCache.Get(ValidateEnforce, nil)) != 0 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, nil)))
	}

	pCache.Add(policy)
	if len(pCache.Get(ValidateEnforce, nil)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, nil)))
	}
}

func Test_Remove_From_Empty_Cache(t *testing.T) {
	pCache := newPolicyCache(log.Log)
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
					"Namespace"
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
					"Namespace"
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
	pCache := newPolicyCache(log.Log)
	policy := newNsPolicy(t)

	// add
	pCache.Add(policy)
	nspace := policy.GetNamespace()
	// get
	if len(pCache.Get(Mutate, &nspace)) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(pCache.Get(Mutate, &nspace)))
	}

	if len(pCache.Get(ValidateEnforce, &nspace)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, &nspace)))
	}

	if len(pCache.Get(Generate, &nspace)) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(pCache.Get(Generate, &nspace)))
	}

	// remove
	pCache.Remove(policy)
	assert.Assert(t, len(pCache.Get(ValidateEnforce, &nspace)) == 0)
}

func Test_Ns_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newNsPolicy(t)

	pCache.Add(policy)
	pCache.Add(policy)
	pCache.Add(policy)
	nspace := policy.GetNamespace()
	if len(pCache.Get(Mutate, &nspace)) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(pCache.Get(Mutate, &nspace)))
	}

	if len(pCache.Get(ValidateEnforce, &nspace)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, &nspace)))
	}

	if len(pCache.Get(Generate, &nspace)) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(pCache.Get(Generate, &nspace)))
	}
}

func Test_Ns_Add_Validate_Audit(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newNsPolicy(t)
	nspace := policy.GetNamespace()

	pCache.Add(policy)
	pCache.Add(policy)

	policy.Spec.ValidationFailureAction = "audit"
	pCache.Add(policy)
	pCache.Add(policy)

	if len(pCache.Get(ValidateEnforce, &nspace)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, &nspace)))
	}

	if len(pCache.Get(ValidateAudit, &nspace)) != 1 {
		t.Errorf("expected 1 validate audit policy, found %v", len(pCache.Get(ValidateAudit, &nspace)))
	}
}

func Test_Ns_Add_Remove(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newNsPolicy(t)

	pCache.Add(policy)
	nspace := policy.GetNamespace()
	if len(pCache.Get(ValidateEnforce, &nspace)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, &nspace)))
	}

	pCache.Remove(policy)
	if len(pCache.Get(ValidateEnforce, &nspace)) != 0 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, &nspace)))
	}

	pCache.Add(policy)
	if len(pCache.Get(ValidateEnforce, &nspace)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce, &nspace)))
	}
}
