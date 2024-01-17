package validatingadmissionpolicygenerate

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/assert"
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
			out, _ := checkResources(res)
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
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			policies, _, err := yamlutils.GetPolicy([]byte(test.policy))
			assert.NilError(t, err)
			assert.Equal(t, 1, len(policies))
			out, _ := canGenerateVAP(policies[0].GetSpec())
			assert.Equal(t, out, test.expected)
		})
	}
}
