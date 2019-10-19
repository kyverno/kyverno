package policy

import (
	"encoding/json"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
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

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = validateUniqueRuleName(*policy)
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

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		err := validateRuleType(rule)
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

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		err := validateRuleType(rule)
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

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		err := validateRuleType(rule)
		assert.NilError(t, err)
	}
}

func Test_Validate_ResourceDescription_Empty(t *testing.T) {
	rawResourcedescirption := []byte(`{}`)

	var rd kyverno.ResourceDescription
	err := json.Unmarshal(rawResourcedescirption, &rd)
	assert.NilError(t, err)

	err = validateMatchedResourceDescription(rd)
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

	var rd kyverno.ResourceDescription
	err := json.Unmarshal(matchedResourcedescirption, &rd)
	assert.NilError(t, err)

	err = validateMatchedResourceDescription(rd)
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

	var rd kyverno.ResourceDescription
	err := json.Unmarshal(matchedResourcedescirption, &rd)
	assert.NilError(t, err)

	err = validateResourceDescription(rd)
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

	var rd kyverno.ResourceDescription
	err := json.Unmarshal(rawResourcedescirption, &rd)
	assert.NilError(t, err)

	err = validateMatchedResourceDescription(rd)
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

	var rd kyverno.ResourceDescription
	err := json.Unmarshal(rawResourcedescirption, &rd)
	assert.NilError(t, err)

	err = validateMatchedResourceDescription(rd)
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

	var rules []kyverno.Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := validateValidation(rule.Validation)
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

	var rules []kyverno.Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := validateValidation(rule.Validation)
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

	var rules []kyverno.Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := validateValidation(rule.Validation)
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

	var rules []kyverno.Rule
	err := json.Unmarshal(rawRules, &rules)
	assert.NilError(t, err)

	for _, rule := range rules {
		errs := validateValidation(rule.Validation)
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

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		errs := validateValidation(rule.Validation)
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

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		errs := validateValidation(rule.Validation)
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

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	for _, rule := range policy.Spec.Rules {
		errs := validateValidation(rule.Validation)
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
				 "name": "validate-runAsNonRoot",
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
				 "name": "validate-allowPrivilegeEscalation",
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

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = Validate(policy)
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

	var mutate kyverno.Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs := validateMutation(mutate)
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

	var mutate kyverno.Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs := validateMutation(mutate)
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

	var mutateExistence kyverno.Mutation
	err := json.Unmarshal(rawMutate, &mutateExistence)
	assert.NilError(t, err)

	errs := validateMutation(mutateExistence)
	assert.Assert(t, len(errs) != 0)

	var mutateEqual kyverno.Mutation
	rawMutate = []byte(`
	{
		"overlay": {
		   "spec": {
			  "=(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutateEqual)
	assert.NilError(t, err)

	errs = validateMutation(mutateEqual)
	assert.Assert(t, len(errs) != 0)

	var mutateNegation kyverno.Mutation
	rawMutate = []byte(`
	{
		"overlay": {
		   "spec": {
			  "X(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutateNegation)
	assert.NilError(t, err)

	errs = validateMutation(mutateNegation)
	assert.Assert(t, len(errs) != 0)
}

// TODO: validate patches
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

	var mutate kyverno.Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	errs := validateMutation(mutate)
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

	errs = validateMutation(mutate)
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

	var validate kyverno.Validation
	err := json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	errs := validateValidation(validate)
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

	var validateNew kyverno.Validation
	err = json.Unmarshal(rawValidateNew, &validateNew)
	assert.NilError(t, err)

	errs = validateValidation(validate)
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

	var validate kyverno.Validation
	err := json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	errs := validateValidation(validate)
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

	var validate kyverno.Validation
	err := json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	errs := validateValidation(validate)
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

	errs = validateValidation(validate)
	assert.Assert(t, len(errs) != 0)
}

func Test_Validate_Generate(t *testing.T) {
	rawGenerate := []byte(`
	{
		"kind": "NetworkPolicy",
		"name": "defaultnetworkpolicy",
		"data": {
		   "spec": {
			  "podSelector": {},
			  "policyTypes": [
				 "Ingress",
				 "Egress"
			  ],
			  "ingress": [
				 {}
			  ],
			  "egress": [
				 {}
			  ]
		   }
		}
	 }`)

	var generate kyverno.Generation
	err := json.Unmarshal(rawGenerate, &generate)
	assert.NilError(t, err)

	err = validateGeneration(generate)
	assert.NilError(t, err)
}

func Test_Validate_Generate_HasAnchors(t *testing.T) {
	rawGenerate := []byte(`
	{
		"kind": "NetworkPolicy",
		"name": "defaultnetworkpolicy",
		"data": {
		   "spec": {
			  "(podSelector)": {},
			  "policyTypes": [
				 "Ingress",
				 "Egress"
			  ],
			  "ingress": [
				 {}
			  ],
			  "egress": [
				 {}
			  ]
		   }
		}
	 }`)

	var generate kyverno.Generation
	err := json.Unmarshal(rawGenerate, &generate)
	assert.NilError(t, err)

	err = validateGeneration(generate)
	assert.Assert(t, err != nil)

	rawGenerateNew := []byte(`
	{
		"kind": "ConfigMap",
		"name": "copied-cm",
		"clone": {
		   "^(namespace)": "default",
		   "name": "game"
		}
	 }`)

	var generateNew kyverno.Generation
	errNew := json.Unmarshal(rawGenerateNew, &generateNew)
	assert.NilError(t, errNew)

	errNew = validateGeneration(generateNew)
	assert.Assert(t, errNew != nil)
}

func Test_Validate_ErrorFormat(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "test-error-format"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "image-pull-policy",
				 "match": {
					"resources": {
					   "kinds": [
						  "Deployment"
					   ],
					   "selector": {
						  "matchLabels": {
							 "app": "nginxlatest"
						  }
					   }
					}
				 },
				 "exclude": {
					"resources": {
						"selector": {
							"app": "nginxlatest"
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
									  "=(image)": "*latest",
									  "imagePullPolicy": "IfNotPresent"
								   }
								]
							 }
						  }
					   }
					}
				 }
			  },
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"resources": {
					   "kinds": [],
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
								"containers": [
								   {
									  "^(securityContext)": {
										 "allowPrivilegeEscalation": "false"
									  }
								   }
								]
							 }
						  }
					   }
					}
				 }
			  },
			  {
				 "name": "default-networkpolicy",
				 "match": {
					"resources": {
					   "kinds": [
						  "Namespace"
					   ],
					   "name": "devtest"
					}
				 },
				 "generate": {
					"kind": "ConfigMap",
					"name": "copied-cm",
					"clone": {
					   "^(namespace)": "default",
					   "name": "game-config"
					}
				 }
			  }
		   ]
		}
	 }
	`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	expectedErr := `
- Invalid Policy 'test-error-format':
duplicate rule name: 'validate-user-privilege'
- invalid rule 'image-pull-policy':
error in exclude block, the requirements are not specified in selector
invalid anchor found at /spec/template/spec/containers/0/=(image), expect: () || +()
- invalid rule 'validate-user-privilege':
error in match block, field Kind is not specified
- invalid rule 'validate-user-privilege':
existing anchor at /spec/template/spec/containers/0/securityContext must be of type array, found: map[string]interface {}
- invalid rule 'default-networkpolicy':
invalid character found on pattern clone: namespace is requried
`
	err = Validate(policy)
	assert.Assert(t, err.Error() == expectedErr)
}
