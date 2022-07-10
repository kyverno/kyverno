package policycache

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	lv1 "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/labels"
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

func fakeHasSynced() bool {
	return true
}

func Test_All(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newPolicy(t)
	//add
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
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
	pCache.remove(policy)
	kind := "pod"
	validateEnforce := pCache.get(ValidateEnforce, kind, "")
	assert.Assert(t, len(validateEnforce) == 0)
}

func Test_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newPolicy(t)
	pCache.add(policy)
	pCache.add(policy)
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
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
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newPolicy(t)
	pCache.add(policy)
	pCache.add(policy)

	policy.Spec.ValidationFailureAction = "audit"
	pCache.add(policy)
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
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
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newPolicy(t)
	kind := "Pod"
	pCache.add(policy)

	validateEnforce := pCache.get(ValidateEnforce, kind, "")
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	mutate := pCache.get(Mutate, kind, "")
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}

	generate := pCache.get(Generate, kind, "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}

	pCache.remove(policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, kind, "")
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_Add_Remove_Any(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newAnyPolicy(t)
	kind := "Pod"
	pCache.add(policy)

	validateEnforce := pCache.get(ValidateEnforce, kind, "")
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	mutate := pCache.get(Mutate, kind, "")
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}

	generate := pCache.get(Generate, kind, "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}

	pCache.remove(policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, kind, "")
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_Remove_From_Empty_Cache(t *testing.T) {
	pCache := newPolicyCache(nil, nil, fakeHasSynced, fakeHasSynced)
	policy := newPolicy(t)

	pCache.remove(policy)
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
				"patchStrategicMerge": {
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

func newAnyPolicy(t *testing.T) *kyverno.ClusterPolicy {
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
						"any": [
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"names": [
										"dev"
									]
								}
							},
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"namespaces": [
										"prod"
									]
								}
							}
						]
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
						"all": [
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"names": [
										"dev"
									]
								}
							},
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"namespaces": [
										"prod"
									]
								}
							}
						]
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
						"any": [
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"names": [
										"dev"
									]
								}
							},
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"namespaces": [
										"prod"
									]
								}
							}
						]
					},
					"mutate": {
						"patchStrategicMerge": {
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
						"any": [
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"names": [
										"dev"
									]
								}
							},
							{
								"resources": {
									"kinds": [
										"Pod"
									],
									"namespaces": [
										"prod"
									]
								}
							}
						]
					},
					"generate": {
						"kind": "NetworkPolicy",
						"name": "default-deny-ingress",
						"namespace": "{{request.object.metadata.name}}",
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

func newNsPolicy(t *testing.T) kyverno.PolicyInterface {
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
				"patchStrategicMerge": {
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

	return policy
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

func newUserTestPolicy(t *testing.T) kyverno.PolicyInterface {
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

	return policy
}

func newgenratePolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"metadata": {
		   "name": "add-networkpolicy",
		   "annotations": {
			  "policies.kyverno.io/title": "Add Network Policy",
			  "policies.kyverno.io/category": "Multi-Tenancy",
			  "policies.kyverno.io/subject": "NetworkPolicy"
		   }
		},
		"spec": {
		   "validationFailureAction": "audit",
		   "rules": [
			  {
				 "name": "default-deny",
				 "match": {
					"resources": {
					   "kinds": [
						  "Namespace"
					   ]
					}
				 },
				 "generate": {
					"kind": "NetworkPolicy",
					"name": "default-deny",
					"namespace": "{{request.object.metadata.name}}",
					"synchronize": true,
					"data": {
					   "spec": {
						  "podSelector": {},
						  "policyTypes": [
							 "Ingress",
							 "Egress"
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
func newMutatePolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"metadata": {
		  "name": "logger-sidecar"
		},
		"spec": {
		  "background": false,
		  "rules": [
			{
			  "match": {
				"resources": {
				  "kinds": [
					"StatefulSet"
				  ]
				}
			  },
			  "mutate": {
				"patchesJson6902": "- op: add\n  path: /spec/template/spec/containers/-1\n  value: {\"name\": \"logger\", \"image\": \"nginx\"}\n- op: add\n  path: /spec/template/spec/volumes/-1\n  value: {\"name\": \"logs\",\"emptyDir\": {\"medium\": \"Memory\"}}\n- op: add\n  path: /spec/template/spec/containers/0/volumeMounts/-1\n  value: {\"mountPath\": \"/opt/app/logs\",\"name\": \"logs\"}"
			  },
			  "name": "logger-sidecar",
			  "preconditions": [
				{
				  "key": "{{ request.object.spec.template.metadata.annotations.\"logger.k8s/inject\"}}",
				  "operator": "Equals",
				  "value": "true"
				},
				{
				  "key": "logger",
				  "operator": "NotIn",
				  "value": "{{ request.object.spec.template.spec.containers[].name }}"
				}
			  ]
			}
		  ],
		  "validationFailureAction": "audit"
		}
	  }`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	return policy
}
func newNsMutatePolicy(t *testing.T) kyverno.PolicyInterface {
	rawPolicy := []byte(`{
		"metadata": {
		  "name": "logger-sidecar",
		  "namespace": "logger"
		},
		"spec": {
		  "background": false,
		  "rules": [
			{
			  "match": {
				"resources": {
				  "kinds": [
					"StatefulSet"
				  ]
				}
			  },
			  "mutate": {
				"patchesJson6902": "- op: add\n  path: /spec/template/spec/containers/-1\n  value: {\"name\": \"logger\", \"image\": \"nginx\"}\n- op: add\n  path: /spec/template/spec/volumes/-1\n  value: {\"name\": \"logs\",\"emptyDir\": {\"medium\": \"Memory\"}}\n- op: add\n  path: /spec/template/spec/containers/0/volumeMounts/-1\n  value: {\"mountPath\": \"/opt/app/logs\",\"name\": \"logs\"}"
			  },
			  "name": "logger-sidecar",
			  "preconditions": [
				{
				  "key": "{{ request.object.spec.template.metadata.annotations.\"logger.k8s/inject\"}}",
				  "operator": "Equals",
				  "value": "true"
				},
				{
				  "key": "logger",
				  "operator": "NotIn",
				  "value": "{{ request.object.spec.template.spec.containers[].name }}"
				}
			  ]
			}
		  ],
		  "validationFailureAction": "audit"
		}
	  }`)

	var policy *kyverno.Policy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	return policy
}

func newValidateAuditPolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"metadata": {
		  "name": "check-label-app-audit"
		},
		"spec": {
		  "background": false,
		  "rules": [
			{
				"match": {
                    "resources": {
                        "kinds": [
                            "Pod"
                        ]
                    }
                },
                "name": "check-label-app",
                "validate": {
                    "message": "The label 'app' is required.",
                    "pattern": {
                        "metadata": {
                            "labels": {
                                "app": "?*"
                            }
                        }
                    }
                }
			}
		  ],
		  "validationFailureAction": "audit",
		  "validationFailureActionOverrides": [
				{
					"action": "enforce",
					"namespaces": [
						"default"
					]
				},
				{
					"action": "audit",
					"namespaces": [
						"test"
					]
				}
			]
		}
	  }`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	return policy
}

func newValidateEnforcePolicy(t *testing.T) *kyverno.ClusterPolicy {
	rawPolicy := []byte(`{
		"metadata": {
		  "name": "check-label-app-enforce"
		},
		"spec": {
		  "background": false,
		  "rules": [
			{
				"match": {
                    "resources": {
                        "kinds": [
                            "Pod"
                        ]
                    }
                },
                "name": "check-label-app",
                "validate": {
                    "message": "The label 'app' is required.",
                    "pattern": {
                        "metadata": {
                            "labels": {
                                "app": "?*"
                            }
                        }
                    }
                }
			}
		  ],
		  "validationFailureAction": "enforce",
		  "validationFailureActionOverrides": [
				{
					"action": "enforce",
					"namespaces": [
						"default"
					]
				},
				{
					"action": "audit",
					"namespaces": [
						"test"
					]
				}
			]
		}
	  }`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	return policy
}

func Test_Ns_All(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newNsPolicy(t)
	//add
	pCache.add(policy)
	nspace := policy.GetNamespace()
	for _, rule := range autogen.ComputeRules(policy) {
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
	pCache.remove(policy)
	kind := "pod"
	validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	assert.Assert(t, len(validateEnforce) == 0)
}

func Test_Ns_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newNsPolicy(t)
	pCache.add(policy)
	pCache.add(policy)
	pCache.add(policy)
	nspace := policy.GetNamespace()
	for _, rule := range autogen.ComputeRules(policy) {
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
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newNsPolicy(t)
	pCache.add(policy)
	pCache.add(policy)
	nspace := policy.GetNamespace()
	policy.GetSpec().ValidationFailureAction = "audit"
	pCache.add(policy)
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
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
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newNsPolicy(t)
	nspace := policy.GetNamespace()
	kind := "Pod"
	pCache.add(policy)
	validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	pCache.remove(policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_GVk_Cache(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newGVKPolicy(t)
	//add
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
		for _, kind := range rule.MatchResources.Kinds {

			generate := pCache.get(Generate, kind, "")
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}
}

func Test_GVK_Add_Remove(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newGVKPolicy(t)
	kind := "ClusterRole"
	pCache.add(policy)
	generate := pCache.get(Generate, kind, "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}

	pCache.remove(policy)
	deletedGenerate := pCache.get(Generate, kind, "")
	if len(deletedGenerate) != 0 {
		t.Errorf("expected 0 generate policy, found %v", len(deletedGenerate))
	}
}

func Test_Add_Validate_Enforce(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newUserTestPolicy(t)
	nspace := policy.GetNamespace()
	//add
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
		for _, kind := range rule.MatchResources.Kinds {
			validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
			if len(validateEnforce) != 1 {
				t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
			}
		}
	}
}

func Test_Ns_Add_Remove_User(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newUserTestPolicy(t)
	nspace := policy.GetNamespace()
	kind := "Deployment"
	pCache.add(policy)
	validateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}

	pCache.remove(policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, kind, nspace)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_Mutate_Policy(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newMutatePolicy(t)
	//add
	pCache.add(policy)
	pCache.add(policy)
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
		for _, kind := range rule.MatchResources.Kinds {

			// get
			mutate := pCache.get(Mutate, kind, "")
			if len(mutate) != 1 {
				t.Errorf("expected 1 mutate policy, found %v", len(mutate))
			}
		}
	}
}

func Test_Generate_Policy(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newgenratePolicy(t)
	//add
	pCache.add(policy)
	for _, rule := range autogen.ComputeRules(policy) {
		for _, kind := range rule.MatchResources.Kinds {

			// get
			generate := pCache.get(Generate, kind, "")
			if len(generate) != 1 {
				t.Errorf("expected 1 generate policy, found %v", len(generate))
			}
		}
	}
}

func Test_NsMutate_Policy(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy := newMutatePolicy(t)
	nspolicy := newNsMutatePolicy(t)
	//add
	pCache.add(policy)
	pCache.add(nspolicy)
	pCache.add(policy)
	pCache.add(nspolicy)

	nspace := policy.GetNamespace()
	// get
	mutate := pCache.get(Mutate, "StatefulSet", "")
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}

	// get
	nsMutate := pCache.get(Mutate, "StatefulSet", nspace)
	if len(nsMutate) != 1 {
		t.Errorf("expected 1 namespace mutate policy, found %v", len(nsMutate))
	}

}

func Test_Validate_Enforce_Policy(t *testing.T) {
	pCache := newPolicyCache(dummyLister{}, dummyNsLister{}, fakeHasSynced, fakeHasSynced)
	policy1 := newValidateAuditPolicy(t)
	policy2 := newValidateEnforcePolicy(t)
	pCache.add(policy1)
	pCache.add(policy2)

	validateEnforce := pCache.get(ValidateEnforce, "Pod", "")
	if len(validateEnforce) != 2 {
		t.Errorf("adding: expected 2 validate enforce policy, found %v", len(validateEnforce))
	}

	validateAudit := pCache.get(ValidateAudit, "Pod", "")
	if len(validateAudit) != 0 {
		t.Errorf("adding: expected 0 validate audit policy, found %v", len(validateAudit))
	}

	pCache.remove(policy1)
	pCache.remove(policy2)

	validateEnforce = pCache.get(ValidateEnforce, "Pod", "")
	if len(validateEnforce) != 0 {
		t.Errorf("removing: expected 0 validate enforce policy, found %v", len(validateEnforce))
	}

	validateAudit = pCache.get(ValidateAudit, "Pod", "")
	if len(validateAudit) != 0 {
		t.Errorf("removing: expected 0 validate audit policy, found %v", len(validateAudit))
	}
}
