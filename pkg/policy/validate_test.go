package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/openapi"
	"gotest.tools/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

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

func Test_Validate_DenyConditions_KeyRequestOperation_Empty(t *testing.T) {
	denyConditions := []byte(`[]`)

	var dcs apiextensions.JSON
	err := json.Unmarshal(denyConditions, &dcs)
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NilError(t, err)
}

func Test_Validate_Preconditions_KeyRequestOperation_Empty(t *testing.T) {
	preConditions := []byte(`[]`)

	var pcs apiextensions.JSON
	err := json.Unmarshal(preConditions, &pcs)
	assert.NilError(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.NilError(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.NilError(t, err)
}

func Test_Validate_DenyConditionsValuesString_KeyRequestOperation_ExpectedValue(t *testing.T) {
	denyConditions := []byte(`
	[
		{
			"key":"{{request.operation}}",
			"operator":"Equals",
			"value":"DELETE"
		},
		{
			"key":"{{request.operation}}",
			"operator":"NotEquals",
			"value":"CREATE"
		},
		{
			"key":"{{request.operation}}",
			"operator":"NotEquals",
			"value":"CONNECT"
		},
		{
			"key":"{{ request.operation }}",
			"operator":"NotEquals",
			"value":"UPDATE"
		},
		{
			"key":"{{lbServiceCount}}",
			"operator":"Equals",
			"value":"2"
		}
	]
	`)

	var dcs apiextensions.JSON
	err := json.Unmarshal(denyConditions, &dcs)
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NilError(t, err)
}

func Test_Validate_DenyConditionsValuesString_KeyRequestOperation_RightfullyTemplatizedValue(t *testing.T) {
	denyConditions := []byte(`
	[
		{
			"key":"{{request.operation}}",
			"operator":"Equals",
			"value":"{{ \"ops-cm\".data.\"deny-ops\"}}"
		},
		{
			"key":"{{ request.operation }}",
			"operator":"NotEquals",
			"value":"UPDATE"
		}
	]
	`)

	var dcs apiextensions.JSON
	err := json.Unmarshal(denyConditions, &dcs)
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NilError(t, err)
}

func Test_Validate_DenyConditionsValuesString_KeyRequestOperation_WrongfullyTemplatizedValue(t *testing.T) {
	denyConditions := []byte(`
	[
		{
			"key":"{{request.operation}}",
			"operator":"Equals",
			"value":"{{ \"ops-cm\".data.\"deny-ops\" }"
		},
		{
			"key":"{{ request.operation }}",
			"operator":"NotEquals",
			"value":"UPDATE"
		}
	]
	`)

	var dcs []kyverno.Condition
	err := json.Unmarshal(denyConditions, &dcs)
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Assert(t, err != nil)
}

func Test_Validate_PreconditionsValuesString_KeyRequestOperation_UnknownValue(t *testing.T) {
	preConditions := []byte(`
	[
		{
			"key":"{{request.operation}}",
			"operator":"Equals",
			"value":"foobar"
		},
		{
			"key": "{{request.operation}}",
			"operator": "NotEquals",
			"value": "CREATE"
		}
	]
	`)

	var pcs apiextensions.JSON
	err := json.Unmarshal(preConditions, &pcs)
	assert.NilError(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Assert(t, err != nil)

	_, err = validateConditions(pcs, "preconditions")
	assert.Assert(t, err != nil)
}

func Test_Validate_DenyConditionsValuesList_KeyRequestOperation_ExpectedItem(t *testing.T) {
	denyConditions := []byte(`
	[
		{
			"key":"{{request.operation}}",
			"operator":"Equals",
			"value": [
				"CREATE",
				"DELETE",
				"CONNECT"
			]
		},
		{
			"key":"{{request.operation}}",
			"operator":"NotEquals",
			"value": [
				"UPDATE"
			]
		},
		{
			"key": "{{lbServiceCount}}",
			"operator": "Equals",
			"value": "2"
		}
	]
	`)

	var dcs []kyverno.Condition
	err := json.Unmarshal(denyConditions, &dcs)
	assert.NilError(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NilError(t, err)
}

func Test_Validate_PreconditionsValuesList_KeyRequestOperation_UnknownItem(t *testing.T) {
	preConditions := []byte(`
	[
		{
			"key":"{{request.operation}}",
			"operator":"Equals",
			"value": [
				"foobar",
				"CREATE"
			]
		},
		{
			"key":"{{request.operation}}",
			"operator":"NotEquals",
			"value": [
				"foobar"
			]
		}
	]
	`)

	var pcs apiextensions.JSON
	err := json.Unmarshal(preConditions, &pcs)
	assert.NilError(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Assert(t, err != nil)

	_, err = validateConditions(pcs, "preconditions")
	assert.Assert(t, err != nil)
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

	openApiManager, _ := openapi.NewManager()
	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	_, err = Validate(policy, nil, true, openApiManager)
	assert.NilError(t, err)
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
					"patchStrategicMerge": {
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

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.Assert(t, err != nil)
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

	err = containsUserVariables(policy, nil)
	assert.Equal(t, err.Error(), "invalid variable used at path: spec/rules[0]/match/roles")
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

	err = containsUserVariables(policy, nil)
	assert.Equal(t, err.Error(), "invalid variable used at path: spec/rules[0]/match/clusterRoles")
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

	err = containsUserVariables(policy, nil)
	assert.Equal(t, err.Error(), "invalid variable used at path: spec/rules[0]/match/subjects")
}

func Test_BackGroundUserInfo_mutate_patchStrategicMerge1(t *testing.T) {
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
				"patchStrategicMerge": {
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

	err = ValidateVariables(policy, true)
	assert.Assert(t, err != nil)
}

func Test_BackGroundUserInfo_mutate_patchStrategicMerge2(t *testing.T) {
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
				"patchStrategicMerge": {
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

	err = ValidateVariables(policy, true)
	assert.Assert(t, err != nil)
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
			  "name": "validate-patch-strategic-merge",
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

	err = ValidateVariables(policy, true)
	assert.Assert(t, err != nil, err)
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

	err = ValidateVariables(policy, true)
	assert.Assert(t, err != nil)
}

func Test_BackGroundUserInfo_validate_anyPattern_multiple_var(t *testing.T) {
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
					"var1": "{{request.userInfo}}-{{temp}}"
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

	err = ValidateVariables(policy, true)
	assert.Assert(t, err != nil)
}

func Test_BackGroundUserInfo_validate_anyPattern_serviceAccount(t *testing.T) {
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
					"var1": "{{serviceAccountName}}"
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

	err = ValidateVariables(policy, true)
	assert.Assert(t, err != nil)
}

func Test_ruleOnlyDealsWithResourceMetaData(t *testing.T) {
	testcases := []struct {
		description    string
		rule           []byte
		expectedOutput bool
	}{
		{
			description:    "Test mutate patchStrategicMerge - pass",
			rule:           []byte(`{"name":"testPatches1","mutate":{"patchStrategicMerge":{"metadata":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}`),
			expectedOutput: true,
		},
		{
			description:    "Test mutate patchStrategicMerge - fail",
			rule:           []byte(`{"name":"testPatches2","mutate":{"patchStrategicMerge":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}`),
			expectedOutput: false,
		},
		{
			description:    "Test mutate patch - pass",
			rule:           []byte(`{"name":"testPatches3","mutate":{"patchesJson6902": "[{\"path\":\"/metadata/labels/isMutated\",\"op\":\"add\",\"value\":\"true\"},{\"path\":\"/metadata/labels/app\",\"op\":\"replace\",\"value\":\"nginx_is_mutated\"}]"}}`),
			expectedOutput: true,
		},
		{
			description:    "Test mutate patch - fail",
			rule:           []byte(`{"name":"testPatches4","mutate":{"patchesJson6902": "[{\"path\":\"/spec/labels/isMutated\",\"op\":\"add\",\"value\":\"true\"},{\"path\":\"/metadata/labels/app\",\"op\":\"replace\",\"value\":\"nginx_is_mutated\"}]" }}`),
			expectedOutput: false,
		},
		{
			description:    "Test validate - pass",
			rule:           []byte(`{"name":"testValidate1","validate":{"message":"CPU and memory resource requests and limits are required","pattern":{"metadata":{"containers":[{"(name)":"*","ports":[{"containerPort":80}]}]}}}}`),
			expectedOutput: true,
		},
		{
			description:    "Test validate - fail",
			rule:           []byte(`{"name":"testValidate2","validate":{"message":"CPU and memory resource requests and limits are required","pattern":{"spec":{"containers":[{"(name)":"*","ports":[{"containerPort":80}]}]}}}}`),
			expectedOutput: false,
		},
		{
			description:    "Test validate any pattern - pass",
			rule:           []byte(`{"name":"testValidateAnyPattern1","validate":{"message":"Volumes white list","anyPattern":[{"metadata":{"volumes":[{"hostPath":"*"}]}},{"metadata":{"volumes":[{"emptyDir":"*"}]}},{"metadata":{"volumes":[{"configMap":"*"}]}}]}}`),
			expectedOutput: true,
		},
		{
			description:    "Test validate any pattern - fail",
			rule:           []byte(`{"name":"testValidateAnyPattern2","validate":{"message":"Volumes white list","anyPattern":[{"spec":{"volumes":[{"hostPath":"*"}]}},{"metadata":{"volumes":[{"emptyDir":"*"}]}},{"metadata":{"volumes":[{"configMap":"*"}]}}]}}`),
			expectedOutput: false,
		},
	}

	for i, testcase := range testcases {
		var rule kyverno.Rule
		_ = json.Unmarshal(testcase.rule, &rule)
		output := ruleOnlyDealsWithResourceMetaData(rule)
		if output != testcase.expectedOutput {
			t.Errorf("Testcase [%d] (%s) failed", i+1, testcase.description)
		}
	}
}

func Test_Validate_Kind(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "policy-to-monitor-root-user-access"
		},
		"spec": {
		  "validationFailureAction": "audit",
		  "rules": [
			{
			  "name": "monitor-annotation-for-root-user-access",
			  "match": {
				"resources": {
				  "selector": {
					"matchLabels": {
					  "AllowRootUserAccess": "true"
					}
				  }
				}
			  },
			  "validate": {
				"message": "Label provisioner.wg.net/cloudprovider is required",
				"pattern": {
				  "metadata": {
					"labels": {
					  "provisioner.wg.net/cloudprovider": "*"
					}
				  }
				}
			  }
			}
		  ]
		}
	  }
	`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.Assert(t, err != nil)
}

func Test_Validate_Any_Kind(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "policy-to-monitor-root-user-access"
		},
		"spec": {
			"validationFailureAction": "audit",
			"rules": [
				{
					"name": "monitor-annotation-for-root-user-access",
					"match": {
						"any": [
							{
								"resources": {
									"selector": {
										"matchLabels": {
											"AllowRootUserAccess": "true"
										}
									}
								}
							}
						]
					},
					"validate": {
						"message": "Label provisioner.wg.net/cloudprovider is required",
						"pattern": {
							"metadata": {
								"labels": {
									"provisioner.wg.net/cloudprovider": "*"
								}
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

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.Assert(t, err != nil)
}

func Test_checkAutoGenRules(t *testing.T) {
	testCases := []struct {
		name           string
		policy         []byte
		expectedResult bool
	}{
		{
			name:           "rule-missing-autogen-cronjob",
			policy:         []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test","annotations":{"pod-policies.kyverno.io/autogen-controllers":"Deployment,CronJob"}},"spec":{"rules":[{"match":{"resources":{"kinds":["Pod"]}},"name":"block-old-flux","validate":{"message":"CannotuseoldFluxv1annotation.","pattern":{"metadata":{"=(annotations)":{"X(fluxcd.io/*)":"*?"}}}}},{"match":{"resources":{"kinds":["Deployment"]}},"name":"autogen-block-old-flux","validate":{"message":"CannotuseoldFluxv1annotation.","pattern":{"spec":{"template":{"metadata":{"=(annotations)":{"X(fluxcd.io/*)":"*?"}}}}}}}]}}`),
			expectedResult: true,
		},
		{
			name:           "rule-missing-autogen-deployment",
			policy:         []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test","annotations":{"pod-policies.kyverno.io/autogen-controllers":"Deployment,CronJob"}},"spec":{"rules":[{"match":{"resources":{"kinds":["Pod"]}},"name":"block-old-flux","validate":{"message":"CannotuseoldFluxv1annotation.","pattern":{"metadata":{"=(annotations)":{"X(fluxcd.io/*)":"*?"}}}}},{"match":{"resources":{"kinds":["CronJob"]}},"name":"autogen-cronjob-block-old-flux","validate":{"message":"CannotuseoldFluxv1annotation.","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"metadata":{"=(annotations)":{"X(fluxcd.io/*)":"*?"}}}}}}}}}]}}`),
			expectedResult: true,
		},
		{
			name:           "rule-missing-autogen-all",
			policy:         []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test","annotations":{"pod-policies.kyverno.io/autogen-controllers":"Deployment,CronJob,StatefulSet,Job,DaemonSet"}},"spec":{"rules":[{"match":{"resources":{"kinds":["Pod"]}},"name":"block-old-flux","validate":{"message":"CannotuseoldFluxv1annotation.","pattern":{"metadata":{"=(annotations)":{"X(fluxcd.io/*)":"*?"}}}}}]}}`),
			expectedResult: true,
		},
		{
			name:           "rule-with-autogen-disabled",
			policy:         []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test","annotations":{"pod-policies.kyverno.io/autogen-controllers":"none"}},"spec":{"rules":[{"match":{"resources":{"kinds":["Pod"]}},"name":"block-old-flux","validate":{"message":"CannotuseoldFluxv1annotation.","pattern":{"metadata":{"=(annotations)":{"X(fluxcd.io/*)":"*?"}}}}}]}}`),
			expectedResult: false,
		},
	}

	for _, test := range testCases {
		var policy kyverno.ClusterPolicy
		err := json.Unmarshal(test.policy, &policy)
		assert.NilError(t, err)

		res := missingAutoGenRules(&policy, logging.GlobalLogger())
		assert.Equal(t, test.expectedResult, res, fmt.Sprintf("test %s failed", test.name))
	}
}

func Test_Validate_ApiCall(t *testing.T) {
	testCases := []struct {
		resource       kyverno.ContextEntry
		expectedResult interface{}
	}{
		{
			resource: kyverno.ContextEntry{
				APICall: &kyverno.APICall{
					URLPath:  "/apis/networking.k8s.io/v1/namespaces/{{request.namespace}}/networkpolicies",
					JMESPath: "",
				},
			},
			expectedResult: nil,
		},
		{
			resource: kyverno.ContextEntry{
				APICall: &kyverno.APICall{
					URLPath:  "/apis/networking.k8s.io/v1/namespaces/{{request.namespace}}/networkpolicies",
					JMESPath: "items[",
				},
			},
			expectedResult: "failed to parse JMESPath items[: SyntaxError: Expected tStar, received: tEOF",
		},
		{
			resource: kyverno.ContextEntry{
				APICall: &kyverno.APICall{
					URLPath:  "/apis/networking.k8s.io/v1/namespaces/{{request.namespace}}/networkpolicies",
					JMESPath: "items[{{request.namespace}}",
				},
			},
			expectedResult: nil,
		},
	}

	for _, testCase := range testCases {
		err := validateAPICall(testCase.resource)

		if err == nil {
			assert.Equal(t, err, testCase.expectedResult)
		} else {
			assert.Equal(t, err.Error(), testCase.expectedResult)
		}
	}
}

func Test_Wildcards_Kind(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "require-labels"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "rules": [
			{
			  "name": "check-for-labels",
			  "match": {
				"resources": {
				  "kinds": [
					"*"
				  ]
				}
			  },
			  "validate": {
				"message": "label 'app.kubernetes.io/name' is required",
				"pattern": {
				  "metadata": {
					"labels": {
					  "app.kubernetes.io/name": "?*"
					}
				  }
				}
			  }
			}
		  ]
		}
	  }
	`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.Assert(t, err != nil)
}

func Test_Namespced_Policy(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "Policy",
		"metadata": {
		  "name": "evil-policy-match-foreign-pods",
		  "namespace": "customer-foo"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "evil-validation",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ],
				  "namespaces": [
					"customer-bar"
				  ]
				}
			  },
			  "validate": {
				"message": "Mua ah ah ... you've been pwned by customer-foo",
				"pattern": {
				  "metadata": {
					"annotations": {
					  "pwned-by-customer-foo": "true"
					}
				  }
				}
			  }
			}
		  ]
		}
	  }
	`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.Assert(t, err != nil)
}

func Test_Namespaced_Generate_Policy(t *testing.T) {
	testcases := []struct {
		description     string
		rule            []byte
		policyNamespace string
		expectedError   error
	}{
		{
			description: "Only generate resource where the policy exists",
			rule: []byte(`
			{"name": "gen-zk",
                "generate": {
                    "synchronize": false,
                    "apiVersion": "v1",
                    "kind": "ConfigMap",
                    "name": "zk",
                    "namespace": "default",
                    "data": {
                        "kind": "ConfigMap",
                        "metadata": {
                            "labels": {
                                "somekey": "somevalue"
                            }
                        },
                        "data": {
                            "ZK_ADDRESS": "192.168.10.10:2181",
                            "KAFKA_ADDRESS": "192.168.10.13:9092"
                        }
                    }
                }
					}`),
			policyNamespace: "poltest",
			expectedError:   errors.New("path: spec.rules[gen-zk]: a namespaced policy cannot generate resources in other namespaces, expected: poltest, received: default"),
		},
		{
			description: "Not allowed to clone resource outside the policy namespace",
			rule: []byte(`
        {
            "name": "sync-image-pull-secret",
            "generate": {
                "apiVersion": "v1",
                "kind": "Secret",
                "name": "secret-basic-auth-gen",
                "namespace": "poltest",
                "synchronize": true,
                "clone": {
                    "namespace": "default",
                    "name": "secret-basic-auth"
                }
            }
        }`),
			policyNamespace: "poltest",
			expectedError:   errors.New("path: spec.rules[sync-image-pull-secret]: a namespaced policy cannot clone resources to or from other namespaces, expected: poltest, received: default"),
		},
		{
			description: "Do not mention the namespace to generate cluster scoped resource",
			rule: []byte(`
        {
            "name": "sync-clone",
            "generate": {
                "apiVersion": "storage.k8s.io/v1",
                "kind": "StorageClass",
                "name": "local-class",
                "namespace": "poltest",
                "synchronize": true,
                "clone": {
                    "name": "pv-class"
                }
            }
        }`),
			policyNamespace: "poltest",
			expectedError:   errors.New("path: spec.rules[sync-clone]: do not mention the namespace to generate a non namespaced resource"),
		},
		{
			description: "Not allowed to clone cluster scoped resource",
			rule: []byte(`
        {
            "name": "sync-clone",
            "generate": {
                "apiVersion": "storage.k8s.io/v1",
                "kind": "StorageClass",
                "name": "local-class",
                "synchronize": true,
                "clone": {
                    "name": "pv-class"
                }
            }
        }`),
			policyNamespace: "poltest",
			expectedError:   errors.New("path: spec.rules[sync-clone]: a namespaced policy cannot generate cluster-wide resources"),
		},
		{
			description: "Not allowed to multi clone cluster scoped resource",
			rule: []byte(`
    {
        "name": "sync-multi-clone",
        "generate": {
            "namespace": "staging",
            "synchronize": true,
            "cloneList": {
                "namespace": "staging",
                "kinds": [
                    "storage.k8s.io/v1/StorageClass"
                ],
                "selector": {
                    "matchLabels": {
                        "allowedToBeCloned": "true"
                    }
                }
            }
        }
    }`),
			policyNamespace: "staging",
			expectedError:   errors.New("path: spec.rules[sync-multi-clone]: a namespaced policy cannot generate cluster-wide resources"),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			var rule kyverno.Rule
			_ = json.Unmarshal(tc.rule, &rule)
			err := checkClusterResourceInMatchAndExclude(rule, sets.NewString(), tc.policyNamespace, false, testResourceList())
			if tc.expectedError != nil {
				assert.Error(t, err, tc.expectedError.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func Test_patchesJson6902_Policy(t *testing.T) {
	rawPolicy := []byte(`
	{
   "apiVersion": "kyverno.io/v1",
   "kind": "ClusterPolicy",
   "metadata": {
      "name": "set-max-surge-yaml-to-json"
   },
   "spec": {
      "background": false,
			"schemaValidation": false,
      "rules": [
         {
            "name": "set-max-surge",
            "context": [
               {
                  "name": "source",
                  "configMap": {
                     "name": "source-yaml-to-json",
                     "namespace": "default"
                  }
               }
            ],
            "match": {
               "resources": {
                  "kinds": [
                     "Deployment"
                  ]
               }
            },
            "mutate": {
               "patchesJson6902": "- op: replace\n  path: /spec/strategy\n  value: {{ source.data.strategy }}"
            }
         }
      ]
   }
}
	`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.NilError(t, err)
}

func Test_deny_exec(t *testing.T) {
	var err error
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "deny-exec-to-pod"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "schemaValidation": false,
		  "rules": [
			{
			  "name": "deny-pod-exec",
			  "match": {
				"resources": {
				  "kinds": [
					"PodExecOptions"
				  ]
				}
			  },
			  "preconditions": {
				"all": [
				  {
					"key": "{{ request.operation }}",
					"operator": "Equals",
					"value": "CONNECT"
				  }
				]
			  },
			  "validate": {
				"message": "Containers can't be exec'd into in production.",
				"deny": {}
			  }
			}
		  ]
		}
	  }`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.NilError(t, err)
}

func Test_existing_resource_policy(t *testing.T) {
	var err error
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "np-test-1"
		},
		"spec": {
		  "validationFailureAction": "audit",
		  "rules": [
			{
			  "name": "no-LoadBalancer",
			  "match": {
				"any": [
				  {
					"resources": {
					  "kinds": [
						"networking.k8s.io/v1/NetworkPolicy"
					  ]
					}
				  }
				]
			  },
			  "validate": {
				"message": "np-test",
				"pattern": {
				  "metadata": {
					"name": "?*"
				  }
				}
			  }
			}
		  ]
		}
	  }`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.NilError(t, err)
}

func Test_PodControllerAutoGenExclusion_All_Controllers_Policy(t *testing.T) {
	rawPolicy := []byte(`
{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "add-all-pod-controller-annotations",
	  "annotations": {
		"pod-policies.kyverno.io/autogen-controllers": "DaemonSet,Job,CronJob,Deployment,StatefulSet"
	  }
	},
	"spec": {
	  "validationFailureAction": "enforce",
	  "background": false,
	  "rules": [
		{
		  "name": "validate-livenessProbe-readinessProbe",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ]
			}
		  },
		  "validate": {
			"message": "Liveness and readiness probes are required.",
			"pattern": {
			  "spec": {
				"containers": [
				  {
					"livenessProbe": {
					  "periodSeconds": ">0"
					},
					"readinessProbe": {
					  "periodSeconds": ">0"
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
`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	res, err := Validate(policy, nil, true, openApiManager)
	assert.NilError(t, err)
	assert.Assert(t, res == nil)
}

func Test_PodControllerAutoGenExclusion_Not_All_Controllers_Policy(t *testing.T) {
	rawPolicy := []byte(`
{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "add-not-all-pod-controller-annotations",
	  "annotations": {
		"pod-policies.kyverno.io/autogen-controllers": "DaemonSet,Job,CronJob,Deployment"
	  }
	},
	"spec": {
	  "validationFailureAction": "enforce",
	  "background": false,
	  "rules": [
		{
		  "name": "validate-livenessProbe-readinessProbe",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ]
			}
		  },
		  "validate": {
			"message": "Liveness and readiness probes are required.",
			"pattern": {
			  "spec": {
				"containers": [
				  {
					"livenessProbe": {
					  "periodSeconds": ">0"
					},
					"readinessProbe": {
					  "periodSeconds": ">0"
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
`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	warnings, err := Validate(policy, nil, true, openApiManager)
	assert.Assert(t, warnings != nil)
	assert.NilError(t, err)
}

func Test_PodControllerAutoGenExclusion_None_Policy(t *testing.T) {
	rawPolicy := []byte(`
{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "add-none-pod-controller-annotations",
	  "annotations": {
		"pod-policies.kyverno.io/autogen-controllers": "none"
	  }
	},
	"spec": {
	  "validationFailureAction": "enforce",
	  "background": false,
	  "rules": [
		{
		  "name": "validate-livenessProbe-readinessProbe",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ]
			}
		  },
		  "validate": {
			"message": "Liveness and readiness probes are required.",
			"pattern": {
			  "spec": {
				"containers": [
				  {
					"livenessProbe": {
					  "periodSeconds": ">0"
					},
					"readinessProbe": {
					  "periodSeconds": ">0"
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
`)

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	warnings, err := Validate(policy, nil, true, openApiManager)
	assert.Assert(t, warnings == nil)
	assert.NilError(t, err)
}

func Test_ValidateNamespace(t *testing.T) {
	testcases := []struct {
		description   string
		spec          *kyverno.Spec
		expectedError error
	}{
		{
			description: "tc1",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("conflicting namespaces found in path: spec.validationFailureActionOverrides[1].namespaces: default"),
		},
		{
			description: "tc2",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Mutation: kyverno.Mutation{
							RawPatchStrategicMerge: &apiextv1.JSON{Raw: []byte(`"metadata": {"labels": {"app-name": "{{request.object.metadata.name}}"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("conflicting namespaces found in path: spec.validationFailureActionOverrides[1].namespaces: default"),
		},
		{
			description: "tc3",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default*",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern 'default*' matches with namespace 'default'"),
		},
		{
			description: "tc4",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern '*' matches with namespace 'default'"),
		},
		{
			description: "tc5",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"?*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern '?*' matches with namespace 'default'"),
		},
		{
			description: "tc6",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default?",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default1",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern 'default?' matches with namespace 'default1'"),
		},
		{
			description: "tc7",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default*",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"?*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern '?*' matches with namespace 'test'"),
		},
		{
			description: "tc8",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"*",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"?*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern '?*' conflicts with the pattern '*'"),
		},
		{
			description: "tc9",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default*",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
							"test*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern 'test*' matches with namespace 'test'"),
		},
		{
			description: "tc10",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"*efault",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
			expectedError: errors.New("path: spec.validationFailureActionOverrides[1].namespaces: wildcard pattern '*efault' matches with namespace 'default'"),
		},
		{
			description: "tc11",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default-*",
							"test",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
		},
		{
			description: "tc12",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default*?",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
							"test*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
		},
		{
			description: "tc13",
			spec: &kyverno.Spec{
				ValidationFailureAction: kyverno.Enforce,
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: kyverno.Enforce,
						Namespaces: []string{
							"default?",
						},
					},
					{
						Action: kyverno.Audit,
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: kyverno.Validation{
							Message:    "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextv1.JSON{Raw: []byte(`"metadata": {"lables": {"app.kubernetes.io/name": "?*"}}`)},
						},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			err := validateNamespaces(tc.spec, field.NewPath("spec").Child("validationFailureActionOverrides"))
			if tc.expectedError != nil {
				assert.Error(t, err, tc.expectedError.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func testResourceList() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "services", Namespaced: true, Kind: "Service"},
				{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
				{Name: "componentstatuses", Namespaced: false, Kind: "ComponentStatus"},
				{Name: "nodes", Namespaced: false, Kind: "Node"},
				{Name: "secrets", Namespaced: true, Kind: "Secret"},
				{Name: "configmaps", Namespaced: true, Kind: "ConfigMap"},
				{Name: "namespacedtype", Namespaced: true, Kind: "NamespacedType"},
				{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
				{Name: "resourcequotas", Namespaced: true, Kind: "ResourceQuota"},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
			},
		},
		{
			GroupVersion: "storage.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "storageclasses", Namespaced: false, Kind: "StorageClass"},
			},
		},
	}
}

func Test_Any_wildcard_policy(t *testing.T) {
	var err error
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "verify-image"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"background": false,
			"rules": [
				{
					"name": "verify-image",
					"match": {
						"any": [
							{
								"resources": {
									"kinds": [
										"*"
									]
								}
							}
						]
					},
					"verifyImages": [
						{
							"imageReferences": [
								"ghcr.io/kyverno/test-verify-image:*"
							],
							"mutateDigest": true,
							"attestors": [
								{
									"entries": [
										{
											"keys": {
												"publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----                \n"
											}
										}
									]
								}
							]
						}
					]
				}
			]
		}
	}`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	openApiManager, _ := openapi.NewManager()
	_, err = Validate(policy, nil, true, openApiManager)
	assert.Assert(t, err != nil)
}
