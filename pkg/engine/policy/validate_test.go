package policy

import (
	"encoding/json"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
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

	_, err = validateUniqueRuleName(*policy)
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
	var err error
	rawResourcedescirption := []byte(`{}`)

	var rd kyverno.ResourceDescription
	err = json.Unmarshal(rawResourcedescirption, &rd)
	assert.NilError(t, err)

	_, err = validateMatchedResourceDescription(rd)
	assert.Assert(t, err != nil)
}

func Test_Validate_ResourceDescription_MatchedValid(t *testing.T) {
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

	_, err = validateMatchedResourceDescription(rd)
	assert.NilError(t, err)
}
func Test_Validate_ResourceDescription_MissingKindsOnMatched(t *testing.T) {
	var err error
	matchedResourcedescirption := []byte(`
	{
		"selector": {
		   "matchLabels": {
			  "app.type": "prod"
		   }
		}
	 }`)

	var rd kyverno.ResourceDescription
	err = json.Unmarshal(matchedResourcedescirption, &rd)
	assert.NilError(t, err)

	_, err = validateMatchedResourceDescription(rd)
	assert.Assert(t, err != nil)
}

func Test_Validate_ResourceDescription_MissingKindsOnExclude(t *testing.T) {
	var err error
	excludeResourcedescirption := []byte(`
	{
		"selector": {
		   "matchLabels": {
			  "app.type": "prod"
		   }
		}
	 }`)

	var rd kyverno.ResourceDescription
	err = json.Unmarshal(excludeResourcedescirption, &rd)
	assert.NilError(t, err)

	_, err = validateExcludeResourceDescription(rd)
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

	err = validateResourceDescription(rd)
	assert.Assert(t, err != nil)
}

func Test_Validate_OverlayPattern_Empty(t *testing.T) {
	rawValidation := []byte(`
   {}`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)

	if _, err := validateValidation(validation); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_OverlayPattern_Nil_PatternAnypattern(t *testing.T) {
	rawValidation := []byte(`
 	{ "message": "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false"
      }
	`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	if _, err := validateValidation(validation); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_OverlayPattern_Exist_PatternAnypattern(t *testing.T) {
	rawValidation := []byte(`
	{
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
	  }`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	if _, err := validateValidation(validation); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_OverlayPattern_Valid(t *testing.T) {
	rawValidation := []byte(`
	{
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
`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	if _, err := validateValidation(validation); err != nil {
		assert.NilError(t, err)
	}

}

func Test_Validate_ExistingAnchor_AnchorOnMap(t *testing.T) {
	rawValidation := []byte(`
	{
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
`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	if _, err := validateValidation(validation); err != nil {
		assert.Assert(t, err != nil)
	}

}

func Test_Validate_ExistingAnchor_AnchorOnString(t *testing.T) {
	rawValidation := []byte(`{
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
	  		  `)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	if _, err := validateValidation(validation); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_ExistingAnchor_Valid(t *testing.T) {
	var err error
	var validation kyverno.Validation
	rawValidation := []byte(`
	{
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
	 }`)

	err = json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	if _, err := validateValidation(validation); err != nil {
		assert.Assert(t, err != nil)
	}
	rawValidation = nil
	rawValidation = []byte(`
	{
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
	 }	`)
	err = json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	if _, err := validateValidation(validation); err != nil {
		assert.Assert(t, err != nil)
	}

}

func Test_Validate_Validate_ValidAnchor(t *testing.T) {
	var err error
	var validate kyverno.Validation
	var rawValidate []byte
	// case 1
	rawValidate = []byte(`
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

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	if _, err := validateValidation(validate); err != nil {
		assert.NilError(t, err)
	}

	// case 2
	rawValidate = nil
	validate = kyverno.Validation{}
	rawValidate = []byte(`
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

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	if _, err := validateValidation(validate); err != nil {
		assert.NilError(t, err)
	}
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
	if _, err := validateValidation(validate); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_Validate_Unsupported(t *testing.T) {
	var err error
	var validate kyverno.Validation

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

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)
	if _, err := validateValidation(validate); err != nil {
		assert.Assert(t, err != nil)
	}

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

	if _, err := validateValidation(validate); err != nil {
		assert.Assert(t, err != nil)
	}

}

func Test_Validate_Policy(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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
	if _, err := validateMutation(mutate); err != nil {
		assert.NilError(t, err)
	}
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

	if _, err := validateMutation(mutate); err != nil {
		assert.NilError(t, err)
	}
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

	if _, err := validateMutation(mutateExistence); err != nil {
		assert.Assert(t, err != nil)
	}

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

	if _, err := validateMutation(mutateEqual); err != nil {
		assert.Assert(t, err != nil)
	}

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

	if _, err := validateMutation(mutateEqual); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_Mutate_Unsupported(t *testing.T) {
	var err error
	var mutate kyverno.Mutation
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

	err = json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	if _, err := validateMutation(mutate); err != nil {
		assert.Assert(t, err != nil)
	}

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

	if _, err := validateMutation(mutate); err != nil {
		assert.Assert(t, err != nil)
	}
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

	if _, err := validateGeneration(generate); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_Generate_HasAnchors(t *testing.T) {
	var err error
	var generate kyverno.Generation
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

	err = json.Unmarshal(rawGenerate, &generate)
	assert.NilError(t, err)
	if _, err := validateGeneration(generate); err != nil {
		assert.Assert(t, err != nil)
	}

	rawGenerate = []byte(`
	{
		"kind": "ConfigMap",
		"name": "copied-cm",
		"clone": {
		   "^(namespace)": "default",
		   "name": "game"
		}
	 }`)

	errNew := json.Unmarshal(rawGenerate, &generate)
	assert.NilError(t, errNew)
	err = json.Unmarshal(rawGenerate, &generate)
	assert.NilError(t, err)
	if _, err := validateGeneration(generate); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_ErrorFormat(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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

	err = Validate(policy)
	assert.Assert(t, err != nil)
}

func Test_Validate_EmptyUserInfo(t *testing.T) {
	rawRule := []byte(`
	{
		"name": "test",
		"match": {
		   "subjects": null
		}
	 }`)

	var rule kyverno.Rule
	err := json.Unmarshal(rawRule, &rule)
	assert.NilError(t, err)

	_, errNew := validateUserInfo(rule)
	assert.NilError(t, errNew)
}

func Test_Validate_Roles(t *testing.T) {
	rawRule := []byte(`{
		"name": "test",
		"match": {
		   "roles": [
			  "namespace1:name1",
			  "name2"
		   ]
		}
	 }`)

	var rule kyverno.Rule
	err := json.Unmarshal(rawRule, &rule)
	assert.NilError(t, err)

	path, err := validateUserInfo(rule)
	assert.Assert(t, err != nil)
	assert.Assert(t, path == "match.roles")
}

func Test_Validate_ServiceAccount(t *testing.T) {
	rawRule := []byte(`
	{
		"name": "test",
		"exclude": {
		   "subjects": [
			  {
				 "kind": "ServiceAccount",
				 "name": "testname"
			  }
		   ]
		}
	 }`)

	var rule kyverno.Rule
	err := json.Unmarshal(rawRule, &rule)
	assert.NilError(t, err)

	path, err := validateUserInfo(rule)
	assert.Assert(t, err != nil)
	assert.Assert(t, path == "exclude.subjects")
}

func Test_BackGroundUserInfo_match_roles(t *testing.T) {
	var err error
	rawPolicy := []byte(`
 {
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "disallow-root-user"
	},
	"spec": {
	  "rules": [
		{
		  "name": "match.roles",
		  "match": {
			"roles": [
			  "a",
			  "b"
			]
		  }
		}
	  ]
	}
  }
 `)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsUserInfo(*policy)

	if err.Error() != "path: spec/rules[0]/match/roles" {
		t.Error("Incorrect Path")
	}
}

func Test_BackGroundUserInfo_match_clusterRoles(t *testing.T) {
	var err error
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "disallow-root-user"
		},
		"spec": {
		  "rules": [
			{
			  "name": "match.clusterRoles",
			  "match": {
				"clusterRoles": [
				  "a",
				  "b"
				]
			  }
			}
		  ]
		}
	  }
 `)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsUserInfo(*policy)

	if err.Error() != "path: spec/rules[0]/match/clusterRoles" {
		t.Log(err)
		t.Error("Incorrect Path")
	}
}

func Test_BackGroundUserInfo_match_subjects(t *testing.T) {
	var err error
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "disallow-root-user"
		},
		"spec": {
		  "rules": [
			{
			  "name": "match.subjects",
			  "match": {
				"subjects": [
				  {
					"Name": "a"
				  },
				  {
					"Name": "b"
				  }
				]
			  }
			}
		  ]
		}
	  } `)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsUserInfo(*policy)

	if err.Error() != "path: spec/rules[0]/match/subjects" {
		t.Log(err)
		t.Error("Incorrect Path")
	}
}

func Test_BackGroundUserInfo_mutate_overlay1(t *testing.T) {
	var err error
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "disallow-root-user"
		},
		"spec": {
		  "rules": [
			{
			  "name": "mutate.overlay1",
			  "mutate": {
				"overlay": {
				  "var1": "{{request.userInfo}}"
				}
			  }
			}
		  ]
		}
	  }
	`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsUserInfo(*policy)

	if err.Error() != "path: spec/rules[0]/mutate/overlay/var1/{{request.userInfo}}" {
		t.Log(err)
		t.Error("Incorrect Path")
	}
}

func Test_BackGroundUserInfo_mutate_overlay2(t *testing.T) {
	var err error
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "disallow-root-user"
		},
		"spec": {
		  "rules": [
			{
			  "name": "mutate.overlay2",
			  "mutate": {
				"overlay": {
				  "var1": "{{request.userInfo.userName}}"
				}
			  }
			}
		  ]
		}
	  }
	`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsUserInfo(*policy)

	if err.Error() != "path: spec/rules[0]/mutate/overlay/var1/{{request.userInfo.userName}}" {
		t.Log(err)
		t.Error("Incorrect Path")
	}
}

func Test_BackGroundUserInfo_validate_pattern(t *testing.T) {
	var err error
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "disallow-root-user"
		},
		"spec": {
		  "rules": [
			{
			  "name": "validate.overlay",
			  "validate": {
				"pattern": {
				  "var1": "{{request.userInfo}}"
				}
			  }
			}
		  ]
		}
	  }
	`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsUserInfo(*policy)

	if err.Error() != "path: spec/rules[0]/validate/pattern/var1/{{request.userInfo}}" {
		t.Log(err)
		t.Error("Incorrect Path")
	}
}

func Test_BackGroundUserInfo_validate_anyPattern(t *testing.T) {
	var err error
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "disallow-root-user"
		},
		"spec": {
		  "rules": [
			{
			  "name": "validate.anyPattern",
			  "validate": {
				"anyPattern": [
				  {
					"var1": "temp"
				  },
				  {
					"var1": "{{request.userInfo}}"
				  }
				]
			  }
			}
		  ]
		}
	  }	`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsUserInfo(*policy)

	if err.Error() != "path: spec/rules[0]/validate/anyPattern[1]/var1/{{request.userInfo}}" {
		t.Log(err)
		t.Error("Incorrect Path")
	}
}
