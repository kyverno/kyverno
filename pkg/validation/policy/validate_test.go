package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	golangassert "gotest.tools/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_PolicyValidationWithInvalidVariable(t *testing.T) {
	policy := &kyverno.ClusterPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterPolicy",
			APIVersion: "kyverno.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "policy-with-invalid-variable",
		},
		Spec: kyverno.Spec{
			Rules: []kyverno.Rule{
				{
					Name: "test-rule-invalid-variable",
					MatchResources: kyverno.MatchResources{
						Any: []kyverno.ResourceFilter{
							{
								ResourceDescription: kyverno.ResourceDescription{
									Kinds: []string{"Pod"},
								},
							},
						},
					},
					Validation: &kyverno.Validation{
						Message: "{{ bar }} world!",
						Deny:    &kyverno.Deny{},
					},
				},
			},
		},
	}

	err := ValidateVariables(policy, false)

	assert.NotNil(t, err)

	assert.Contains(t, err.Error(), "variable substitution failed")
	assert.Contains(t, err.Error(), "test-rule-invalid-variable")
	assert.Contains(t, err.Error(), "variable bar must match regex")
}

func Test_Validate_ResourceDescription_Empty(t *testing.T) {
	var err error
	rawResourcedescirption := []byte(`{}`)

	var rd kyverno.ResourceDescription
	err = json.Unmarshal(rawResourcedescirption, &rd)
	assert.Nil(t, err)

	_, err = validateMatchedResourceDescription(rd)
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	_, err = validateMatchedResourceDescription(rd)
	assert.Nil(t, err)
}

func Test_Validate_DenyConditions_KeyRequestOperation_Empty(t *testing.T) {
	denyConditions := []byte(`[]`)

	var dcs apiextensions.JSON
	err := json.Unmarshal(denyConditions, &dcs)
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Nil(t, err)
}

func Test_Validate_Preconditions_KeyRequestOperation_Empty(t *testing.T) {
	preConditions := []byte(`[]`)

	var pcs apiextensions.JSON
	err := json.Unmarshal(preConditions, &pcs)
	assert.Nil(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Nil(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Nil(t, err)
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
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Nil(t, err)
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
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Nil(t, err)
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
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Nil(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Nil(t, err)
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
	assert.Nil(t, err)

	_, err = validateConditions(dcs, "conditions")
	assert.Nil(t, err)
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
	assert.Nil(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Nil(t, err)

	_, err = validateConditions(pcs, "preconditions")
	assert.Nil(t, err)
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

	var policy *kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.Nil(t, err)
}

// needed mock to be true
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

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
	assert.Nil(t, err)

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
	assert.Nil(t, err)

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
	assert.Nil(t, err)

	err = ValidateVariables(policy, true)
	assert.NotNil(t, err)
}

func Test_Context_Variable_Substitution(t *testing.T) {
	var err error
	rawPolicy := []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "check-images"
  },
  "spec": {
    "validationFailureAction": "Enforce",
    "webhookTimeoutSeconds": 30,
    "rules": [
      {
        "name": "call-aws-signer-extension",
        "match": {
          "any": [
            {
              "resources": {
                "namespaces": [
                  "test-notation"
                ],
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "context": [
          {
            "name": "response",
            "apiCall": {
              "method": "POST",
              "data": [
                {
                  "key": "imagesInfo",
                  "value": "{{ images }}"
                }
              ],
              "service": {
                "url": "https://svc.kyverno-notation-aws/checkimages",
                "caBundle": "-----BEGIN CERTIFICATE-----\nMIICizCCAjGgAwIBAgIRAIUEJcm7TtwJEtRtsI2yUcMwCgYIKoZIzj0EAwIwGzEZ\nMBcGA1UEAxMQbXktc2VsZnNpZ25lZC1jYTAeFw0yMzA1MTAwNTI5MzBaFw0yMzA4\nMDgwNTI5MzBaMDExEDAOBgNVBAoTB25pcm1hdGExHTAbBgNVBAMTFGt5dmVybm8t\nbm90YXRpb24tYXdzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxrSr\nVXUbuQbF4rhh0/jqDE6agtXqS9jko6vHTEZUF2Y9f0LdSycEdCocIKZmPerWER7l\nVUmMFPQLSGOZrCIM22L9+EXDyL7q2PN3koDxKOyqVOod8j3hKdRL+KIiZuUeD4zD\ncos+AFxA1XAM/220JKfPSUpBL0DAP299Baqjs/Ae5wU5wT4qZVa1I3pcV2uicPvE\nRSZO3ZT+y1nYBWtTTzzXP3f9ou8IHweCl57Sk16mbFFZ+TrCSekewYchzn88z7lq\nL+56LtBUjcJozypLGEWM+kc4S5wBNYUaFPGiCHIrdQ5ScmfnY7mDvO8u47E+xw13\nbz7NUAlT73rBqBv6hQIDAQABo3UwczAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYB\nBQUHAwIwDAYDVR0TAQH/BAIwADAfBgNVHSMEGDAWgBRXZIp2KalD6pjRfPua2kFn\nMBuJJTAjBgNVHREEHDAaghhzdmMua3l2ZXJuby1ub3RhdGlvbi1hd3MwCgYIKoZI\nzj0EAwIDSAAwRQIhAKob5SV/N56VqP8VPdHqCAULRj92qhWwW3hb7fzaGxnHAiBP\n3c8K2Vrxx2KRsjnWwn1vUMz7UyM2Tmib1C4YM3f+xg==\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBdjCCAR2gAwIBAgIRAP1VDXD3R744lE7t/I5MK44wCgYIKoZIzj0EAwIwGzEZ\nMBcGA1UEAxMQbXktc2VsZnNpZ25lZC1jYTAeFw0yMzA1MTAwNTI5MjVaFw0yMzA4\nMDgwNTI5MjVaMBsxGTAXBgNVBAMTEG15LXNlbGZzaWduZWQtY2EwWTATBgcqhkjO\nPQIBBggqhkjOPQMBBwNCAAQrFCRBF8PjKPcT/lrXmyP474fNuhlhGFAlLaoTSUuP\nS3VK2O7hWrlJ/AhCccY8EPBi/DdFEaCB2+hTo00clmvfo0IwQDAOBgNVHQ8BAf8E\nBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUV2SKdimpQ+qY0Xz7mtpB\nZzAbiSUwCgYIKoZIzj0EAwIDRwAwRAIgU3O7Qnk9PGCV4aXgZAXp0h4Iz2O7XUnP\nUfv4SgD7neECIHLb+BDvRFPJ77FpfIYxBO70AHB7Kp0nWKCqyv3FK4aT\n-----END CERTIFICATE-----"
              }
            }
          }
        ],
        "validate": {
          "message": "not allowed",
          "deny": {
            "conditions": {
              "all": [
                {
                  "key": "{{ response.verified }}",
                  "operator": "EQUALS",
                  "value": false
                }
              ]
            }
          }
        }
      }
    ]
  }
}`)
	var policy *kyverno.ClusterPolicy
	err = json.Unmarshal(rawPolicy, &policy)
	assert.Nil(t, err)

	err = ValidateVariables(policy, true)
	assert.Nil(t, err)
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
	assert.Nil(t, err)

	err = ValidateVariables(policy, true)
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	err = ValidateVariables(policy, true)
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	err = ValidateVariables(policy, true)
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	err = ValidateVariables(policy, true)
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	err = ValidateVariables(policy, true)
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.NotNil(t, err)
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

	var policy *kyverno.Policy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.NotNil(t, err)
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.Nil(t, err)
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.Nil(t, err)
}

func Test_SignatureAlgorithm(t *testing.T) {
	testcases := []struct {
		description    string
		policy         []byte
		expectedOutput bool
	}{
		{
			description: "Test empty signature algorithm - pass",
			policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "check-empty-signature-algorithm"
				},
				"spec": {
					"rules": [
						{
							"match": {
								"resources": {
									"kinds": [
										"Pod"
									]
								}
							},
							"verifyImages": [
								{
									"imageReferences": [
										"ghcr.io/kyverno/test-verify-image:*"
									],
									"attestors": [
										{
											"count": 1,
											"entries": [
												{
													"keys": {
														"publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"
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
			}`),
			expectedOutput: true,
		},
		{
			description: "Test invalid signature algorithm - fail",
			policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "check-invalid-signature-algorithm"
				},
				"spec": {
					"rules": [
						{
							"match": {
								"resources": {
									"kinds": [
										"Pod"
									]
								}
							},
							"verifyImages": [
								{
									"imageReferences": [
										"ghcr.io/kyverno/test-verify-image:*"
									],
									"attestors": [
										{
											"count": 1,
											"entries": [
												{
													"keys": {
														"publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----",
														"signatureAlgorithm": "sha123"
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
			}`),
			expectedOutput: false,
		},
		{
			description: "Test invalid signature algorithm - fail",
			policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "check-valid-signature-algorithm"
				},
				"spec": {
					"rules": [
						{
							"match": {
								"resources": {
									"kinds": [
										"Pod"
									]
								}
							},
							"verifyImages": [
								{
									"imageReferences": [
										"ghcr.io/kyverno/test-verify-image:*"
									],
									"attestors": [
										{
											"count": 1,
											"entries": [
												{
													"keys": {
														"publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----",
														"signatureAlgorithm": "sha256"
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
			}`),
			expectedOutput: true,
		},
	}
	for _, testcase := range testcases {
		var policy *kyverno.ClusterPolicy
		err := json.Unmarshal(testcase.policy, &policy)
		assert.Nil(t, err)

		_, err = Validate(policy, nil, nil, true, "", "")
		if testcase.expectedOutput {
			assert.Nil(t, err)
		} else {
			assert.ErrorContains(t, err, "Invalid signature algorithm provided")
		}
	}
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.Nil(t, err)
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
	  "validationFailureAction": "Enforce",
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
	assert.Nil(t, err)

	res, err := Validate(policy, nil, nil, true, "", "")
	assert.Nil(t, err)
	assert.Nil(t, res)
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
	  "validationFailureAction": "Enforce",
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
	assert.Nil(t, err)

	warnings, err := Validate(policy, nil, nil, true, "", "")
	assert.NotNil(t, warnings)
	assert.Nil(t, err)
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
	  "validationFailureAction": "Enforce",
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
	assert.Nil(t, err)

	warnings, err := Validate(policy, nil, nil, true, "", "")
	assert.Nil(t, warnings)
	assert.Nil(t, err)
}

func Test_ValidateJSON6902(t *testing.T) {
	var patch string = `- path: "/metadata/labels/img"
  op: addition
  value: "nginx"`
	err := validateJSONPatch(patch, 0)
	assert.NotNil(t, err)

	patch = `- path: "/metadata/labels/img"
  op: add
  value: "nginx"`
	err = validateJSONPatch(patch, 0)
	assert.Nil(t, err)

	patch = `- path: "/metadata/labels/img"
  op: add
  value: "nginx"`
	err = validateJSONPatch(patch, 0)
	assert.Nil(t, err)
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Mutation: &kyverno.Mutation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default*",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"?*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default?",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"default1",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default*",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"?*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"*",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"?*",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default*",
							"test",
						},
					},
					{
						Action: "Audit",
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
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"*efault",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default-*",
							"test",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default*?",
						},
					},
					{
						Action: "Audit",
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
						Validation: &kyverno.Validation{
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
				ValidationFailureAction: "Enforce",
				ValidationFailureActionOverrides: []kyverno.ValidationFailureActionOverride{
					{
						Action: "Enforce",
						Namespaces: []string{
							"default?",
						},
					},
					{
						Action: "Audit",
						Namespaces: []string{
							"default",
						},
					},
				},
				Rules: []kyverno.Rule{
					{
						Name:           "require-labels",
						MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"Pod"}}},
						Validation: &kyverno.Validation{
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
			err := validateNamespaces(tc.spec.ValidationFailureActionOverrides, field.NewPath("spec").Child("validationFailureActionOverrides"))
			if tc.expectedError != nil {
				assert.Error(t, err, tc.expectedError.Error())
			} else {
				assert.Nil(t, err)
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
	assert.Nil(t, err)

	_, err = Validate(policy, nil, nil, true, "", "")
	assert.NotNil(t, err)
}

func Test_Validate_RuleImageExtractorsJMESPath(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "jmes-path-and-mutate-digest"
		},
		"spec": {
			"rules": [
				{
					"match": {
						"resources": {
							"kinds": [
								"CRD"
							]
						}
					},
					"imageExtractors": {
						"CRD": [
							{
								"path": "/path/to/image/prefixed/with/scheme",
								"jmesPath": "trim_prefix(@, 'docker://')"
							}
						]
					},
					"verifyImages": [
						{
							"mutateDigest": true,
							"attestors": [
								{
									"count": 1,
									"entries": [
										{
											"keys": {
												"publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.Nil(t, err)

	expectedErr := fmt.Errorf("path: spec.rules[0]: jmespath may not be used in an image extractor when mutating digests with verify images")

	_, actualErr := Validate(policy, nil, nil, true, "", "")
	assert.Equal(t, expectedErr.Error(), actualErr.Error())
}

func Test_GenerateFieldsUpdates(t *testing.T) {
	tests := []struct {
		name          string
		oldPolicy     []byte
		newPolicy     []byte
		expectedErr   bool
		expectWarning bool
	}{
		{
			name: "update-apiVersion",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "apps/v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-kind",
			oldPolicy: []byte(`
{
    "apiVersion": "kyverno.io/v2beta1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "cpol-clone-sync-modify-source"
    },
    "spec": {
        "rules": [
            {
                "name": "cpol-clone-sync-modify-source-secret",
                "match": {
                    "any": [
                        {
                            "resources": {
                                "kinds": [
                                    "Namespace"
                                ]
                            }
                        }
                    ]
                },
                "generate": {
                    "apiVersion": "v1",
                    "kind": "Secret",
                    "name": "regcred",
                    "namespace": "{{request.object.metadata.name}}",
                    "synchronize": true,
                    "clone": {
                        "namespace": "default",
                        "name": "regcred"
                    }
                }
            }
        ]
    }
}`),
			newPolicy: []byte(`
{
    "apiVersion": "kyverno.io/v2beta1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "cpol-clone-sync-modify-source"
    },
    "spec": {
        "rules": [
            {
                "name": "cpol-clone-sync-modify-source-secret",
                "match": {
                    "any": [
                        {
                            "resources": {
                                "kinds": [
                                    "Namespace"
                                ]
                            }
                        }
                    ]
                },
                "generate": {
                    "apiVersion": "v1",
                    "kind": "Configmap",
                    "name": "regcred",
                    "namespace": "{{request.object.metadata.name}}",
                    "synchronize": true,
                    "clone": {
                        "namespace": "default",
                        "name": "regcred"
                    }
                }
            }
        ]
    }
}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-namespace",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.labels.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-name",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "new-name",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-sync-flag",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": false,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: false,
		},
		{
			name: "update-match-statement-with-synchronizing-rule",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-match"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-match-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}
			`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-match"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-match-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"ConfigMap"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}
			`),
			expectedErr:   true,
			expectWarning: false,
		},
		{
			name: "update-match-statement-with-no-synchronizing-rule",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-no-sync-modify-match"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-no-sync-modify-match-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": false,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}
			`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-no-sync-modify-match"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-no-sync-modify-match-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"ConfigMap"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": false,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}
			`),
			expectedErr:   false,
			expectWarning: false,
		},
		{
			name: "update-clone-name",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "modifed-name"
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-clone-namespace",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "default",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "modifed-namespace",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-clone-namespace-unset-new",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"namespace": "prod",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "cpol-clone-sync-modify-source"
				},
				"spec": {
					"rules": [
						{
							"name": "cpol-clone-sync-modify-source-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "regcred",
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"clone": {
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-cloneList-kinds",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									]
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret"
									]
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-cloneList-namespace",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									]
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "prod",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									]
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-cloneList-selector",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									],
									"selector": {
										"matchLabels": {
											"allowedToBeCloned": "true"
										}
									}
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									],
									"selector": {
										"matchLabels": {
											"allowedToBeCloned": "false"
										}
									}
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-clone-List-selector-unset",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									],
									"selector": {
										"matchLabels": {
											"allowedToBeCloned": "true"
										}
									}
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									]
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: true,
		},
		{
			name: "update-cloneList-selector-nochange",
			oldPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									],
									"selector": {
										"matchLabels": {
											"allowedToBeCloned": "true"
										}
									}
								}
							}
						}
					]
				}
			}`),
			newPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "sync-with-multi-clone"
				},
				"spec": {
					"generateExisting": false,
					"rules": [
						{
							"name": "sync-secret",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Namespace"
											]
										}
									}
								]
							},
							"generate": {
								"namespace": "{{request.object.metadata.name}}",
								"synchronize": true,
								"cloneList": {
									"namespace": "default",
									"kinds": [
										"v1/Secret",
										"v1/ConfigMap"
									],
									"selector": {
										"matchLabels": {
											"allowedToBeCloned": "true"
										}
									}
								}
							}
						}
					]
				}
			}`),
			expectedErr:   false,
			expectWarning: false,
		},
	}

	for _, test := range tests {
		var old, new *kyverno.Policy
		err := json.Unmarshal(test.oldPolicy, &old)
		assert.Nil(t, err)
		err = json.Unmarshal(test.newPolicy, &new)
		assert.Nil(t, err)

		warning, err := immutableGenerateFields(new, old)
		golangassert.Assert(t, (warning != "") == test.expectWarning, "%s: %v", test.name, err)
		golangassert.Assert(t, (err != nil) == test.expectedErr, "%s: %v", test.name, err)

	}
}

func Test_isMapStringString(t *testing.T) {
	type args struct {
		m map[string]interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "nil",
		args: args{
			m: nil,
		},
		want: true,
	}, {
		name: "empty",
		args: args{
			m: map[string]interface{}{},
		},
		want: true,
	}, {
		name: "string values",
		args: args{
			m: map[string]interface{}{
				"a": "b",
				"c": "d",
			},
		},
		want: true,
	}, {
		name: "int value",
		args: args{
			m: map[string]interface{}{
				"a": "b",
				"c": 123,
			},
		},
		want: false,
	}, {
		name: "nil value",
		args: args{
			m: map[string]interface{}{
				"a": "b",
				"c": nil,
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMapStringString(tt.args.m); got != tt.want {
				t.Errorf("checkLabelAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}
