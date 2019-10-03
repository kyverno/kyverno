package v1alpha1

import (
	"encoding/json"
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func Test_Validate_UniqueRuleName(t *testing.T) {
	rawPolicy := []byte(`
	{
		"spec": {
		   "validationFailureAction": "audit",
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
				 "validate": {}
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
				 "validate": {}
			  }
		   ]
		}
	 }
	`)

	var policy *ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = policy.ValidateUniqueRuleName()
	assert.Assert(t, err != nil)
}

func Test_Validate_RuleType_EmptyRule(t *testing.T) {
	rawPolicy := []byte(`
	{
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "mutate": {},
				 "validate": {}
			  }
		   ]
		}
	 }
	`)

	var policy *ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		err := rule.ValidateRuleType()
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_RuleType_MultipleRule(t *testing.T) {
	rawPolicy := []byte(`
	{
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "mutate": {
					"overlay": {
					   "spec": {
						  "template": {
							 "spec": {
								"containers": [
								   {
									  "(name)": "*",
									  "resources": {
										 "limits": {
											"+(memory)": "300Mi",
											"+(cpu)": "100"
										 }
									  }
								   }
								]
							 }
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"anyPattern": [
					   {
						  "spec": {
							 "template": {
								"spec": {
								   "containers": [
									  {
										 "securityContext": {
											"runAsNonRoot": true
										 }
									  }
								   ]
								}
							 }
						  }
					   }
					]
				 }
			  }
		   ]
		}
	 }`)

	var policy *ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		err := rule.ValidateRuleType()
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_RuleType_SingleRule(t *testing.T) {
	rawPolicy := []byte(`
	{
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"anyPattern": [
					   {
						  "spec": {
							 "template": {
								"spec": {
								   "containers": [
									  {
										 "securityContext": {
											"runAsNonRoot": "true"
										 }
									  }
								   ]
								}
							 }
						  }
					   }
					]
				 }
			  }
		   ]
		}
	 }
	`)

	var policy *ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		err := rule.ValidateRuleType()
		assert.NilError(t, err)
	}
}

func Test_Validate_ResourceDescription_Empty(t *testing.T) {
	rawResourcedescirption := []byte(`{}`)

	var rd *ResourceDescription
	err := json.Unmarshal(rawResourcedescirption, &rd)
	assert.NilError(t, err)

	err = rd.Validate(true)
	assert.NilError(t, err)
}

func Test_Validate_ResourceDescription_MissingKindsOnMatched(t *testing.T) {
	matchedResourcedescirption := []byte(`
	{
		"selector": {
		   "matchLabels": {
			  "app.type": "prod"
		   }
		}
	 }`)

	var rd *ResourceDescription
	err := json.Unmarshal(matchedResourcedescirption, &rd)
	assert.NilError(t, err)

	err = rd.Validate(true)
	assert.Assert(t, err != nil)
}

func Test_Validate_ResourceDescription_MissingKindsOnExclude(t *testing.T) {
	matchedResourcedescirption := []byte(`
	{
		"selector": {
		   "matchLabels": {
			  "app.type": "prod"
		   }
		}
	 }`)

	var rd *ResourceDescription
	err := json.Unmarshal(matchedResourcedescirption, &rd)
	assert.NilError(t, err)

	err = rd.Validate(false)
	assert.NilError(t, err)
}

func Test_Validate_ResourceDescription_InvalidSelector(t *testing.T) {
	rawResourcedescirption := []byte(`
	{
		"kinds": [
		   "Deployment"
		],
		"selector": {
		   "app.type": "prod"
		}
	 }`)

	var rd *ResourceDescription
	err := json.Unmarshal(rawResourcedescirption, &rd)
	assert.NilError(t, err)

	err = rd.Validate(true)
	assert.Assert(t, err != nil)
}

func Test_Validate_ResourceDescription_Valid(t *testing.T) {
	rawResourcedescirption := []byte(`
	{
		"kinds": [
		   "Deployment"
		],
		"selector": {
		   "matchLabels": {
			  "app.type": "prod"
		   }
		}
	 }`)

	var rd *ResourceDescription
	err := json.Unmarshal(rawResourcedescirption, &rd)
	assert.NilError(t, err)

	err = rd.Validate(true)
	assert.NilError(t, err)
}

func Test_Validate_OverlayPattern_Empty(t *testing.T) {
	rawRules := []byte(`
	[
   {
      "name": "deny-privileged-disallowpriviligedescalation",
      "match": {
         "resources": {
            "kinds": [
               "Pod"
            ]
         }
      },
      "validate": {}
   }
]`)

	var rules []Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := rule.Validation.Validate()
		assert.Assert(t, len(errs) == 0)
	}
}

func Test_Validate_OverlayPattern_Nil_PatternAnypattern(t *testing.T) {
	rawRules := []byte(`
	[
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
         "message": "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false"
      }
   }
]
	`)

	var rules []Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := rule.Validation.Validate()
		assert.Assert(t, len(errs) != 0)
	}
}

func Test_Validate_OverlayPattern_Exist_PatternAnypattern(t *testing.T) {
	rawRules := []byte(`
	[
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
         "message": "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false",
         "anyPattern": [
            {
               "spec": {
                  "securityContext": {
                     "allowPrivilegeEscalation": false,
                     "privileged": false
                  }
               }
            }
         ],
         "pattern": {
            "spec": {
               "containers": [
                  {
                     "name": "*",
                     "securityContext": {
                        "allowPrivilegeEscalation": false,
                        "privileged": false
                     }
                  }
               ]
            }
         }
      }
   }
]`)

	var rules []Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := rule.Validation.Validate()
		assert.Assert(t, len(errs) != 0)
	}
}

func Test_Validate_OverlayPattern_Valid(t *testing.T) {
	rawRules := []byte(`
	[
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
         "message": "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false",
         "anyPattern": [
            {
               "spec": {
                  "securityContext": {
                     "allowPrivilegeEscalation": false,
                     "privileged": false
                  }
               }
            },
            {
               "spec": {
                  "containers": [
                     {
                        "name": "*",
                        "securityContext": {
                           "allowPrivilegeEscalation": false,
                           "privileged": false
                        }
                     }
                  ]
               }
            }
         ]
      }
   }
]`)

	var rules []Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := rule.Validation.Validate()
		assert.Assert(t, len(errs) == 0)
	}
}

func Test_Validate_ExistingAnchor_AnchorOnMap(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "container-security-context"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"anyPattern": [
					   {
						  "spec": {
							 "template": {
								"spec": {
								   "containers": [
									  {
										 "^(securityContext)": {
											"runAsNonRoot": true
										 }
									  }
								   ]
								}
							 }
						  }
					   }
					]
				 }
			  }
		   ]
		}
	 }`)

	var policy ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		errs := rule.Validation.Validate()
		assert.Assert(t, len(errs) == 1)
	}
}

func Test_Validate_ExistingAnchor_AnchorOnString(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "container-security-context"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"pattern": {
					   "spec": {
						  "template": {
							 "spec": {
								"containers": [
								   {
									  "securityContext": {
										 "allowPrivilegeEscalation": "^(false)"
									  }
								   }
								]
							 }
						  }
					   }
					}
				 }
			  }
		   ]
		}
	 }`)

	var policy ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		errs := rule.Validation.Validate()
		assert.Assert(t, len(errs) == 1)
	}
}

func Test_Validate_ExistingAnchor_Valid(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "container-security-context"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"anyPattern": [
					   {
						  "spec": {
							 "template": {
								"spec": {
								   "^(containers)": [
									  {
										 "securityContext": {
											"runAsNonRoot": "true"
										 }
									  }
								   ]
								}
							 }
						  }
					   }
					]
				 }
			  },
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"pattern": {
					   "spec": {
						  "template": {
							 "spec": {
								"^(containers)": [
								   {
									  "securityContext": {
										 "allowPrivilegeEscalation": "false"
									  }
								   }
								]
							 }
						  }
					   }
					}
				 }
			  }
		   ]
		}
	 }`)

	var policy ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		errs := rule.Validation.Validate()
		assert.Assert(t, len(errs) == 0)
	}
}

func Test_Validate_Policy(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "container-security-context"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "exclude": {
					"resources": {
					   "namespaces": [
						  "kube-system"
					   ]
					}
				 },
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app.type": "prod"
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"anyPattern": [
					   {
						  "spec": {
							 "template": {
								"spec": {
								   "^(containers)": [
									  {
										 "securityContext": {
											"runAsNonRoot": true
										 }
									  }
								   ]
								}
							 }
						  }
					   }
					]
				 }
			  }
		   ]
		}
	 }`)

	var policy ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = policy.Validate()
	assert.NilError(t, err)
}

func Test_Validate_Mutate_ConditionAnchor(t *testing.T) {
	rawMutate := []byte(`
	{
		"overlay": {
		   "spec": {
			  "(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	var mutate Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs := mutate.Validate()
	assert.Assert(t, len(errs) == 0)
}

func Test_Validate_Mutate_PlusAnchor(t *testing.T) {
	rawMutate := []byte(`
	{
		"overlay": {
		   "spec": {
			  "+(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	var mutate Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs := mutate.Validate()
	assert.Assert(t, len(errs) == 0)
}

func Test_Validate_Mutate_Mismatched(t *testing.T) {
	rawMutate := []byte(`
	{
		"overlay": {
		   "spec": {
			  "^(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	var mutate Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs := mutate.Validate()
	assert.Assert(t, len(errs) != 0)

	rawMutate = []byte(`
	{
		"overlay": {
		   "spec": {
			  "=(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs = mutate.Validate()
	assert.Assert(t, len(errs) != 0)
}

func Test_Validate_Mutate_Unsupported(t *testing.T) {
	// case 1
	rawMutate := []byte(`
	{
		"overlay": {
		   "spec": {
			  "!(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	var mutate Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs := mutate.Validate()
	assert.Assert(t, len(errs) != 0)

	// case 2
	rawMutate = []byte(`
	{
		"overlay": {
		   "spec": {
			  "~(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs = mutate.Validate()
	assert.Assert(t, len(errs) != 0)
}

func Test_Validate_Validate_ValidAnchor(t *testing.T) {
	// case 1
	rawValidate := []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"anyPattern": [
		   {
			  "spec": {
				 "securityContext": {
					"(runAsNonRoot)": true
				 }
			  }
		   },
		   {
			  "spec": {
				 "^(containers)": [
					{
					   "name": "*",
					   "securityContext": {
						  "runAsNonRoot": true
					   }
					}
				 ]
			  }
		   }
		]
	 }`)

	var validate Validation
	err := json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	errs := validate.Validate()
	assert.Assert(t, len(errs) == 0)

	// case 2
	rawValidateNew := []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "=(securityContext)": {
				 "runAsNonRoot": "true"
			  }
		   }
		}
	}`)

	var validateNew Validation
	err = json.Unmarshal(rawValidateNew, &validateNew)
	assert.NilError(t, err)

	errs = validate.Validate()
	fmt.Println(errs)
	assert.Assert(t, len(errs) == 0)
}

func Test_Validate_Validate_Mismatched(t *testing.T) {
	rawValidate := []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "containers": [
				 {
					"name": "*",
					"securityContext": {
					   "+(runAsNonRoot)": true
					}
				 }
			  ]
		   }
		}
	 }`)

	var validate Validation
	err := json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	errs := validate.Validate()
	assert.Assert(t, len(errs) != 0)

}

func Test_Validate_Validate_Unsupported(t *testing.T) {
	// case 1
	rawValidate := []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "containers": [
				 {
					"name": "*",
					"securityContext": {
					   "!(runAsNonRoot)": true
					}
				 }
			  ]
		   }
		}
	}`)

	var validate Validation
	err := json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	errs := validate.Validate()
	assert.Assert(t, len(errs) != 0)

	// case 2
	rawValidate = []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "containers": [
				 {
					"name": "*",
					"securityContext": {
					   "~(runAsNonRoot)": true
					}
				 }
			  ]
		   }
		}
	}`)

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	errs = validate.Validate()
	assert.Assert(t, len(errs) != 0)
}
