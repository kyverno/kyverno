package admissionpolicy

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Check_Resources(t *testing.T) {
	testCases := []struct {
		name     string
		resource []byte
		expected bool
	}{
		{
			name: "resource-with-namespaces",
			resource: []byte(`
{
  "kinds": [
    "Service"
  ],
  "namespaces": [
    "prod"
  ],
  "operations": [
    "CREATE"
  ]
}
`),
			expected: true,
		},
		{
			name: "namespaces-with-wildcards",
			resource: []byte(`
{
  "kinds": [
    "Service"
  ],
  "namespaces": [
    "prod-*"
  ],
  "operations": [
    "CREATE"
  ]
}
`),
			expected: false,
		},
		{
			name: "resource-names-with-wildcards",
			resource: []byte(`
{
  "kinds": [
    "Service"
  ],
  "names": [
    "svc-*"
  ],
  "operations": [
    "CREATE"
  ]
}
`),
			expected: false,
		},
		{
			name: "resource-with-annotations",
			resource: []byte(`
{
  "annotations": {
    "imageregistry": "https://hub.docker.com/"
  },
  "kinds": [
    "Pod"
  ],
  "operations": [
    "CREATE",
    "UPDATE"
  ]
}
`),
			expected: false,
		},
		{
			name: "resource-with-object-selector",
			resource: []byte(`
{
  "kinds": [
    "Pod"
  ],
  "operations": [
    "CREATE",
    "UPDATE"
  ],
  "selector": {
    "matchLabels": {
      "app": "critical"
    }
  }
}
`),
			expected: true,
		},
		{
			name: "resource-with-namespace-selector",
			resource: []byte(`
{
  "kinds": [
    "Pod"
  ],
  "operations": [
    "CREATE",
    "UPDATE"
  ],
  "namespaceSelector": {
    "matchLabels": {
      "app": "critical"
    }
  }
}
`),
			expected: true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var res kyvernov1.ResourceDescription
			err := json.Unmarshal(test.resource, &res)
			assert.NilError(t, err)
			out, _ := checkResources(res, true)
			assert.Equal(t, out, test.expected)
		})
	}
}

func Test_Check_Exception(t *testing.T) {
	testCases := []struct {
		name       string
		exceptions []kyvernov2.PolicyException
		expected   bool
	}{
		{
			name: "exception-with-multiple-policies",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1"},
							},
							{
								PolicyName: "test-2",
								RuleNames:  []string{"rule-2"},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "exception-with-multiple-rules",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1", "rule-2"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "exception-with-multiple-rules-in-different-exceptions",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1", "rule-2"},
							},
							{
								PolicyName: "test-2",
								RuleNames:  []string{"rule-1"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "exception-with-conditions",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1"},
							},
						},
						Conditions: &kyvernov2.AnyAllConditions{
							AllConditions: []kyvernov2.Condition{
								{
									RawKey: &kyverno.Any{
										Value: "{{ request.object.name }}",
									},
									Operator: kyvernov2.ConditionOperators["Equals"],
									RawValue: &kyverno.Any{
										Value: "dummy",
									},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "exception-with-multiple-all",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1"},
							},
						},
						Match: kyvernov2beta1.MatchResources{
							All: kyvernov1.ResourceFilters{
								kyvernov1.ResourceFilter{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds:      []string{"Pod"},
										Operations: []kyvernov1.AdmissionOperation{"CREATE"},
									},
								},
								kyvernov1.ResourceFilter{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds:      []string{"Pod"},
										Operations: []kyvernov1.AdmissionOperation{"CREATE"},
									},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "exception-with-namespace-selector",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1"},
							},
						},
						Match: kyvernov2beta1.MatchResources{
							Any: kyvernov1.ResourceFilters{
								kyvernov1.ResourceFilter{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds:      []string{"Pod"},
										Operations: []kyvernov1.AdmissionOperation{"CREATE"},
										NamespaceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "critical",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "exception-with-object-selector",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1"},
							},
						},
						Match: kyvernov2beta1.MatchResources{
							Any: kyvernov1.ResourceFilters{
								kyvernov1.ResourceFilter{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds:      []string{"Pod"},
										Operations: []kyvernov1.AdmissionOperation{"CREATE"},
										Selector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "critical",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "exception-with-multiple-any",
			exceptions: []kyvernov2.PolicyException{
				{
					Spec: kyvernov2.PolicyExceptionSpec{
						Exceptions: []kyvernov2.Exception{
							{
								PolicyName: "test-1",
								RuleNames:  []string{"rule-1"},
							},
						},
						Match: kyvernov2beta1.MatchResources{
							Any: kyvernov1.ResourceFilters{
								kyvernov1.ResourceFilter{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds:      []string{"Pod"},
										Operations: []kyvernov1.AdmissionOperation{"CREATE"},
									},
								},
								kyvernov1.ResourceFilter{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds:      []string{"Pod"},
										Operations: []kyvernov1.AdmissionOperation{"CREATE"},
									},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			out, _ := checkExceptions(test.exceptions)
			assert.Equal(t, out, test.expected)
		})
	}
}

func Test_Can_Generate_ValidatingAdmissionPolicy(t *testing.T) {
	testCases := []struct {
		name     string
		policy   []byte
		expected bool
	}{
		{
			name: "policy-with-two-rules",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-latest-tag"
  },
  "spec": {
    "rules": [
      {
        "name": "require-image-tag",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "validate": {
          "cel": {
            "expressions": [
              {
                "expression": "object.spec.containers.all(container, !container.image.matches('^[a-zA-Z]+:[0-9]*$'))"
              }
            ]
          }
        }
      },
      {
        "name": "validate-image-tag",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "validate": {
          "cel": {
            "expressions": [
              {
                "expression": "object.spec.containers.all(container, !container.image.contains('latest'))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-mutate-rule",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "set-image-pull-policy"
  },
  "spec": {
    "rules": [
      {
        "name": "set-image-pull-policy",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "mutate": {
          "patchStrategicMerge": {
            "spec": {
              "containers": [
                {
                  "(image)": "*:latest",
                  "imagePullPolicy": "IfNotPresent"
                }
              ]
            }
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-non-CEL-validate-rule",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "require-ns-purpose-label"
  },
  "spec": {
    "validationFailureAction": "Enforce",
    "rules": [
      {
        "name": "require-ns-purpose-label",
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
        "validate": {
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
}
`),
			expected: false,
		},
		{
			name: "policy-with-multiple-validationFailureActionOverrides",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-host-path"
  },
  "spec": {
    "validationFailureAction": "Enforce",
    "validationFailureActionOverrides": [
      {
        "action": "Enforce",
        "namespaces": [
          "default"
        ]
      },
      {
        "action": "Audit",
        "namespaces": [
          "test"
        ]
      }
    ],
    "rules": [
      {
        "name": "host-path",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "validate": {
          "cel": {
            "expressions": [
              {
                "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-namespace-in-validationFailureActionOverrides",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-host-path"
  },
  "spec": {
    "validationFailureAction": "Enforce",
    "validationFailureActionOverrides": [
      {
        "action": "Enforce",
        "namespaces": [
          "test-ns"
        ]
      }
    ],
    "rules": [
      {
        "name": "host-path",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "validate": {
          "cel": {
            "expressions": [
              {
                "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-multiple-validationFailureActionOverrides-in-validate-rule",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-host-path"
  },
  "spec": {
    "rules": [
      {
        "name": "host-path",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "validate": {
          "failureAction": "Enforce",
          "failureActionOverrides": [
            {
              "action": "Enforce",
              "namespaces": [
                "default"
              ]
            },
            {
              "action": "Audit",
              "namespaces": [
                "test"
              ]
            }
          ],
          "cel": {
            "expressions": [
              {
                "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-namespace-in-validationFailureActionOverrides-in-validate-rule",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-host-path"
  },
  "spec": {
    "rules": [
      {
        "name": "host-path",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "validate": {
          "failureAction": "Enforce",
          "failureActionOverrides": [
            {
              "action": "Enforce",
              "namespaces": [
                "test-ns"
              ]
            }
          ],
          "cel": {
            "expressions": [
              {
                "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-subjects-and-clusterroles",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-host-path"
  },
  "spec": {
    "validationFailureAction": "Enforce",
    "rules": [
      {
        "name": "host-path",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Deployment"
                ],
                "operations": [
                  "CREATE",
                  "UPDATE"
                ]
              },
              "subjects": [
                {
                  "kind": "User",
                  "name": "mary@somecorp.com"
                }
              ],
              "clusterRoles": [
                "cluster-admin"
              ]
            }
          ]
        },
        "validate": {
          "cel": {
            "expressions": [
              {
                "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-object-selector",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-host-path"
  },
  "spec": {
    "validationFailureAction": "Enforce",
    "rules": [
      {
        "name": "host-path",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Deployment"
                ],
                "operations": [
                  "CREATE",
                  "UPDATE"
                ],
                "selector": {
                  "matchLabels": {
                    "app": "mongodb"
                  },
                  "matchExpressions": [
                    {
                      "key": "tier",
                      "operator": "In",
                      "values": [
                        "database"
                      ]
                    }
                  ]
                }
              }
            }
          ]
        },
        "validate": {
          "cel": {
            "generate": true,
            "expressions": [
              {
                "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: true,
		},
		{
			name: "policy-with-generate-set-to-false",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "disallow-host-path"
  },
  "spec": {
    "validationFailureAction": "Enforce",
    "rules": [
      {
        "name": "host-path",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Deployment"
                ],
                "operations": [
                  "CREATE",
                  "UPDATE"
                ],
                "selector": {
                  "matchLabels": {
                    "app": "mongodb"
                  },
                  "matchExpressions": [
                    {
                      "key": "tier",
                      "operator": "In",
                      "values": [
                        "database"
                      ]
                    }
                  ]
                }
              }
            }
          ]
        },
        "validate": {
          "cel": {
            "generate": false,
            "expressions": [
              {
                "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))"
              }
            ]
          }
        }
      }
    ]
  }
}
`),
			expected: false,
		},
		{
			name: "policy-with-no-rules",
			policy: []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "empty-policy"
  },
  "spec": {
    "rules": []
  }
}`),
			expected: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			policies, _, _, _, _, _, _, err := yamlutils.GetPolicy([]byte(test.policy))
			assert.NilError(t, err)
			assert.Equal(t, 1, len(policies))
			out, _ := CanGenerateVAP(policies[0].GetSpec(), nil, false)
			assert.Equal(t, out, test.expected)
		})
	}
}
