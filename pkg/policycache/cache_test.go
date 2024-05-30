package policycache

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
	kubecache "k8s.io/client-go/tools/cache"
)

func setPolicy(t *testing.T, store store, policy kyvernov1.PolicyInterface, finder ResourceFinder) {
	key, _ := kubecache.MetaNamespaceKeyFunc(policy)
	err := store.set(key, policy, finder)
	assert.NilError(t, err)
}

func unsetPolicy(store store, policy kyvernov1.PolicyInterface) {
	key, _ := kubecache.MetaNamespaceKeyFunc(policy)
	store.unset(key)
}

func Test_All(t *testing.T) {
	pCache := newPolicyCache()
	policy := newPolicy(t)
	finder := TestResourceFinder{}
	//add
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				// get
				mutate := pCache.get(Mutate, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(mutate) != 1 {
					t.Errorf("expected 1 mutate policy, found %v", len(mutate))
				}
				validateEnforce := pCache.get(ValidateEnforce, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(validateEnforce) != 1 {
					t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
				}
				generate := pCache.get(Generate, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(generate) != 1 {
					t.Errorf("expected 1 generate policy, found %v", len(generate))
				}
			}
		}
	}

	// remove
	unsetPolicy(pCache, policy)
	validateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	assert.Assert(t, len(validateEnforce) == 0)
}

func Test_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache()
	policy := newPolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				mutate := pCache.get(Mutate, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(mutate) != 1 {
					t.Errorf("expected 1 mutate policy, found %v", len(mutate))
				}

				validateEnforce := pCache.get(ValidateEnforce, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(validateEnforce) != 1 {
					t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
				}
				generate := pCache.get(Generate, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(generate) != 1 {
					t.Errorf("expected 1 generate policy, found %v", len(generate))
				}
			}
		}
	}
}

func Test_Add_Validate_Audit(t *testing.T) {
	pCache := newPolicyCache()
	policy := newPolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	policy.Spec.ValidationFailureAction = "audit"
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				validateEnforce := pCache.get(ValidateEnforce, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(validateEnforce) != 0 {
					t.Errorf("expected 0 validate (enforce) policy, found %v", len(validateEnforce))
				}

				validateAudit := pCache.get(ValidateAudit, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(validateAudit) != 1 {
					t.Errorf("expected 1 validate (audit) policy, found %v", len(validateAudit))
				}
			}
		}
	}
}

func Test_Add_Remove(t *testing.T) {
	pCache := newPolicyCache()
	policy := newPolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy, finder)
	validateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}
	mutate := pCache.get(Mutate, podsGVRS.GroupVersionResource(), "", "")
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}
	generate := pCache.get(Generate, podsGVRS.GroupVersionResource(), "", "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}
	unsetPolicy(pCache, policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_Add_Remove_Any(t *testing.T) {
	pCache := newPolicyCache()
	policy := newAnyPolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy, finder)
	validateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}
	mutate := pCache.get(Mutate, podsGVRS.GroupVersionResource(), "", "")
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}
	generate := pCache.get(Generate, podsGVRS.GroupVersionResource(), "", "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}
	unsetPolicy(pCache, policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_Remove_From_Empty_Cache(t *testing.T) {
	pCache := newPolicyCache()
	policy := newPolicy(t)
	unsetPolicy(pCache, policy)
}

func newPolicy(t *testing.T) *kyvernov1.ClusterPolicy {
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
	var policy *kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newAnyPolicy(t *testing.T) *kyvernov1.ClusterPolicy {
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
	var policy *kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newNsPolicy(t *testing.T) kyvernov1.PolicyInterface {
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
	var policy *kyvernov1.Policy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newGVKPolicy(t *testing.T) *kyvernov1.ClusterPolicy {
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
	var policy *kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newUserTestPolicy(t *testing.T) kyvernov1.PolicyInterface {
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
	var policy *kyvernov1.Policy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newGeneratePolicy(t *testing.T) *kyvernov1.ClusterPolicy {
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
	var policy *kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newMutatePolicy(t *testing.T) *kyvernov1.ClusterPolicy {
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
	var policy *kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newNsMutatePolicy(t *testing.T) kyvernov1.PolicyInterface {
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
	var policy *kyvernov1.Policy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newValidateAuditPolicy(t *testing.T) *kyvernov1.ClusterPolicy {
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
	var policy *kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func newValidateEnforcePolicy(t *testing.T) *kyvernov1.ClusterPolicy {
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
	var policy *kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	return policy
}

func Test_Ns_All(t *testing.T) {
	pCache := newPolicyCache()
	policy := newNsPolicy(t)
	finder := TestResourceFinder{}
	//add
	setPolicy(t, pCache, policy, finder)
	nspace := policy.GetNamespace()
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				// get
				mutate := pCache.get(Mutate, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(mutate) != 1 {
					t.Errorf("expected 1 mutate policy, found %v", len(mutate))
				}
				validateEnforce := pCache.get(ValidateEnforce, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(validateEnforce) != 1 {
					t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
				}
				generate := pCache.get(Generate, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(generate) != 1 {
					t.Errorf("expected 1 generate policy, found %v", len(generate))
				}
			}
		}
	}
	// remove
	unsetPolicy(pCache, policy)
	validateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", nspace)
	assert.Assert(t, len(validateEnforce) == 0)
}

func Test_Ns_Add_Duplicate_Policy(t *testing.T) {
	pCache := newPolicyCache()
	policy := newNsPolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	nspace := policy.GetNamespace()
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				mutate := pCache.get(Mutate, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(mutate) != 1 {
					t.Errorf("expected 1 mutate policy, found %v", len(mutate))
				}
				validateEnforce := pCache.get(ValidateEnforce, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(validateEnforce) != 1 {
					t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
				}
				generate := pCache.get(Generate, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(generate) != 1 {
					t.Errorf("expected 1 generate policy, found %v", len(generate))
				}
			}
		}
	}
}

func Test_Ns_Add_Validate_Audit(t *testing.T) {
	pCache := newPolicyCache()
	policy := newNsPolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	nspace := policy.GetNamespace()
	policy.GetSpec().ValidationFailureAction = "audit"
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				validateEnforce := pCache.get(ValidateEnforce, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(validateEnforce) != 0 {
					t.Errorf("expected 0 validate (enforce) policy, found %v", len(validateEnforce))
				}

				validateAudit := pCache.get(ValidateAudit, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(validateAudit) != 1 {
					t.Errorf("expected 1 validate (audit) policy, found %v", len(validateAudit))
				}
			}
		}
	}
}

func Test_Ns_Add_Remove(t *testing.T) {
	pCache := newPolicyCache()
	policy := newNsPolicy(t)
	finder := TestResourceFinder{}
	nspace := policy.GetNamespace()
	setPolicy(t, pCache, policy, finder)
	validateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}
	unsetPolicy(pCache, policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", nspace)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_GVk_Cache(t *testing.T) {
	pCache := newPolicyCache()
	policy := newGVKPolicy(t)
	finder := TestResourceFinder{}
	//add
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				generate := pCache.get(Generate, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(generate) != 1 {
					t.Errorf("expected 1 generate policy, found %v", len(generate))
				}
			}
		}
	}
}

func Test_GVK_Add_Remove(t *testing.T) {
	pCache := newPolicyCache()
	policy := newGVKPolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy, finder)
	generate := pCache.get(Generate, clusterrolesGVRS.GroupVersionResource(), "", "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}
	unsetPolicy(pCache, policy)
	deletedGenerate := pCache.get(Generate, clusterrolesGVRS.GroupVersionResource(), "", "")
	if len(deletedGenerate) != 0 {
		t.Errorf("expected 0 generate policy, found %v", len(deletedGenerate))
	}
}

func Test_Add_Validate_Enforce(t *testing.T) {
	pCache := newPolicyCache()
	policy := newUserTestPolicy(t)
	nspace := policy.GetNamespace()
	finder := TestResourceFinder{}
	//add
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				validateEnforce := pCache.get(ValidateEnforce, gvr.GroupVersionResource(), gvr.SubResource, nspace)
				if len(validateEnforce) != 1 {
					t.Errorf("expected 1 validate policy, found %v", len(validateEnforce))
				}
			}
		}
	}
}

func Test_Ns_Add_Remove_User(t *testing.T) {
	pCache := newPolicyCache()
	policy := newUserTestPolicy(t)
	nspace := policy.GetNamespace()
	finder := TestResourceFinder{}
	// kind := "Deployment"
	setPolicy(t, pCache, policy, finder)
	validateEnforce := pCache.get(ValidateEnforce, deploymentsGVRS.GroupVersionResource(), "", nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}
	unsetPolicy(pCache, policy)
	deletedValidateEnforce := pCache.get(ValidateEnforce, deploymentsGVRS.GroupVersionResource(), "", nspace)
	if len(deletedValidateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(deletedValidateEnforce))
	}
}

func Test_Mutate_Policy(t *testing.T) {
	pCache := newPolicyCache()
	policy := newMutatePolicy(t)
	finder := TestResourceFinder{}
	//add
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				// get
				mutate := pCache.get(Mutate, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(mutate) != 1 {
					t.Errorf("expected 1 mutate policy, found %v", len(mutate))
				}
			}
		}
	}
}

func Test_Generate_Policy(t *testing.T) {
	pCache := newPolicyCache()
	policy := newGeneratePolicy(t)
	finder := TestResourceFinder{}
	//add
	setPolicy(t, pCache, policy, finder)
	for _, rule := range autogen.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.Kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
			gvrs, err := finder.FindResources(group, version, kind, subresource)
			assert.NilError(t, err)
			for gvr := range gvrs {
				// get
				generate := pCache.get(Generate, gvr.GroupVersionResource(), gvr.SubResource, "")
				if len(generate) != 1 {
					t.Errorf("expected 1 generate policy, found %v", len(generate))
				}
			}
		}
	}
}

func Test_NsMutate_Policy(t *testing.T) {
	pCache := newPolicyCache()
	policy := newMutatePolicy(t)
	nspolicy := newNsMutatePolicy(t)
	finder := TestResourceFinder{}
	//add
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, nspolicy, finder)
	setPolicy(t, pCache, policy, finder)
	setPolicy(t, pCache, nspolicy, finder)
	nspace := policy.GetNamespace()
	// get
	mutate := pCache.get(Mutate, statefulsetsGVRS.GroupVersionResource(), "", "")
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}
	// get
	nsMutate := pCache.get(Mutate, statefulsetsGVRS.GroupVersionResource(), "", nspace)
	if len(nsMutate) != 1 {
		t.Errorf("expected 1 namespace mutate policy, found %v", len(nsMutate))
	}
}

func Test_Validate_Enforce_Policy(t *testing.T) {
	pCache := newPolicyCache()
	policy1 := newValidateAuditPolicy(t)
	policy2 := newValidateEnforcePolicy(t)
	finder := TestResourceFinder{}
	setPolicy(t, pCache, policy1, finder)
	setPolicy(t, pCache, policy2, finder)
	validateEnforce := pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	if len(validateEnforce) != 2 {
		t.Errorf("adding: expected 2 validate enforce policy, found %v", len(validateEnforce))
	}
	validateAudit := pCache.get(ValidateAudit, podsGVRS.GroupVersionResource(), "", "")
	if len(validateAudit) != 0 {
		t.Errorf("adding: expected 0 validate audit policy, found %v", len(validateAudit))
	}
	unsetPolicy(pCache, policy1)
	unsetPolicy(pCache, policy2)
	validateEnforce = pCache.get(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	if len(validateEnforce) != 0 {
		t.Errorf("removing: expected 0 validate enforce policy, found %v", len(validateEnforce))
	}
	validateAudit = pCache.get(ValidateAudit, podsGVRS.GroupVersionResource(), "", "")
	if len(validateAudit) != 0 {
		t.Errorf("removing: expected 0 validate audit policy, found %v", len(validateAudit))
	}
}

func Test_Get_Policies(t *testing.T) {
	cache := NewCache()
	policy := newPolicy(t)
	finder := TestResourceFinder{}
	key, _ := kubecache.MetaNamespaceKeyFunc(policy)
	cache.Set(key, policy, finder)
	validateAudit := cache.GetPolicies(ValidateAudit, namespacesGVRS.GroupVersionResource(), "", "")
	if len(validateAudit) != 0 {
		t.Errorf("expected 0 validate audit policy, found %v", len(validateAudit))
	}
	validateAudit = cache.GetPolicies(ValidateAudit, podsGVRS.GroupVersionResource(), "", "test")
	if len(validateAudit) != 0 {
		t.Errorf("expected 0 validate audit policy, found %v", len(validateAudit))
	}
	validateEnforce := cache.GetPolicies(ValidateEnforce, namespacesGVRS.GroupVersionResource(), "", "")
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}
	mutate := cache.GetPolicies(Mutate, podsGVRS.GroupVersionResource(), "", "")
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}
	generate := cache.GetPolicies(Generate, podsGVRS.GroupVersionResource(), "", "")
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}
}

func Test_Get_Policies_Ns(t *testing.T) {
	cache := NewCache()
	policy := newNsPolicy(t)
	finder := TestResourceFinder{}
	key, _ := kubecache.MetaNamespaceKeyFunc(policy)
	cache.Set(key, policy, finder)
	nspace := policy.GetNamespace()
	validateAudit := cache.GetPolicies(ValidateAudit, podsGVRS.GroupVersionResource(), "", nspace)
	if len(validateAudit) != 0 {
		t.Errorf("expected 0 validate audit policy, found %v", len(validateAudit))
	}
	validateEnforce := cache.GetPolicies(ValidateEnforce, podsGVRS.GroupVersionResource(), "", nspace)
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}
	mutate := cache.GetPolicies(Mutate, podsGVRS.GroupVersionResource(), "", nspace)
	if len(mutate) != 1 {
		t.Errorf("expected 1 mutate policy, found %v", len(mutate))
	}
	generate := cache.GetPolicies(Generate, podsGVRS.GroupVersionResource(), "", nspace)
	if len(generate) != 1 {
		t.Errorf("expected 1 generate policy, found %v", len(generate))
	}
}

func Test_Get_Policies_Validate_Failure_Action_Overrides(t *testing.T) {
	cache := NewCache()
	policy1 := newValidateAuditPolicy(t)
	policy2 := newValidateEnforcePolicy(t)
	finder := TestResourceFinder{}
	key1, _ := kubecache.MetaNamespaceKeyFunc(policy1)
	cache.Set(key1, policy1, finder)
	key2, _ := kubecache.MetaNamespaceKeyFunc(policy2)
	cache.Set(key2, policy2, finder)
	validateAudit := cache.GetPolicies(ValidateAudit, podsGVRS.GroupVersionResource(), "", "")
	if len(validateAudit) != 1 {
		t.Errorf("expected 1 validate audit policy, found %v", len(validateAudit))
	}
	validateEnforce := cache.GetPolicies(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "")
	if len(validateEnforce) != 1 {
		t.Errorf("expected 1 validate enforce policy, found %v", len(validateEnforce))
	}
	validateAudit = cache.GetPolicies(ValidateAudit, podsGVRS.GroupVersionResource(), "", "test")
	if len(validateAudit) != 2 {
		t.Errorf("expected 2 validate audit policy, found %v", len(validateAudit))
	}
	validateEnforce = cache.GetPolicies(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "test")
	if len(validateEnforce) != 0 {
		t.Errorf("expected 0 validate enforce policy, found %v", len(validateEnforce))
	}
	validateAudit = cache.GetPolicies(ValidateAudit, podsGVRS.GroupVersionResource(), "", "default")
	if len(validateAudit) != 0 {
		t.Errorf("expected 0 validate audit policy, found %v", len(validateAudit))
	}
	validateEnforce = cache.GetPolicies(ValidateEnforce, podsGVRS.GroupVersionResource(), "", "default")
	if len(validateEnforce) != 2 {
		t.Errorf("expected 2 validate enforce policy, found %v", len(validateEnforce))
	}
}
