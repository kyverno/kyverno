package policycache

import (
	"encoding/json"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Test_All(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newPolicy(t)

	// add
	pCache.Add(policy)

	// get
	if len(pCache.Get(Mutate)) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(pCache.Get(Mutate)))
	}

	if len(pCache.Get(ValidateEnforce)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce)))
	}

	if len(pCache.Get(Generate)) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(pCache.Get(Generate)))
	}

	// remove
	pCache.Remove(policy)
	assert.Assert(t, len(pCache.Get(ValidateEnforce)) == 0)
}

func Test_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newPolicy(t)

	pCache.Add(policy)
	pCache.Add(policy)
	pCache.Add(policy)

	if len(pCache.Get(Mutate)) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(pCache.Get(Mutate)))
	}

	if len(pCache.Get(ValidateEnforce)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce)))
	}

	if len(pCache.Get(Generate)) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(pCache.Get(Generate)))
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

	if len(pCache.Get(ValidateEnforce)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce)))
	}

	if len(pCache.Get(ValidateAudit)) != 1 {
		t.Errorf("expected 1 validate audit policy, found %v", len(pCache.Get(ValidateAudit)))
	}
}

func Test_Add_Remove(t *testing.T) {
	pCache := newPolicyCache(log.Log)
	policy := newPolicy(t)

	pCache.Add(policy)
	if len(pCache.Get(ValidateEnforce)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce)))
	}

	pCache.Remove(policy)
	if len(pCache.Get(ValidateEnforce)) != 0 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce)))
	}

	pCache.Add(policy)
	if len(pCache.Get(ValidateEnforce)) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(pCache.Get(ValidateEnforce)))
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
				  "conditions": [
					{
					  "key": "a",
					  "operator": "Equals",
					  "value": "a"
					}
				  ]
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
