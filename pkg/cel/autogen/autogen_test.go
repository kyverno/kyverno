package autogen

import (
	"encoding/json"
	"fmt"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Test_CanAutoGen(t *testing.T) {
	testCases := []struct {
		name                string
		policy              []byte
		expectedControllers sets.Set[string]
	}{
		{
			name: "policy-with-match-name",
			policy: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "ValidatingPolicy",
    "metadata": {
        "name": "chech-labels"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        ""
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE",
                        "UPDATE"
                    ],
                    "resources": [
                        "pods"
                    ],
                    "resourceNames": [
                        "test-pod"
                    ]
                }
            ]
        },
        "variables": [
            {
                "name": "environment",
                "expression": "has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == 'prod'"
            }
        ],
        "validations": [
            {
                "expression": "variables.environment == true",
                "message": "Pod labels must be env=prod"
            }
        ]
    }
}`),
			expectedControllers: sets.New("none"),
		},
		{
			name: "policy-with-match-object-selector",
			policy: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "ValidatingPolicy",
    "metadata": {
        "name": "chech-labels"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        ""
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE",
                        "UPDATE"
                    ],
                    "resources": [
                        "pods"
                    ]
                }
            ],
            "objectSelector": {
                "matchLabels": {
                    "app": "nginx"
                }
            }
        },
        "variables": [
            {
                "name": "environment",
                "expression": "has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == 'prod'"
            }
        ],
        "validations": [
            {
                "expression": "variables.environment == true",
                "message": "Pod labels must be env=prod"
            }
        ]
    }
}`),
			expectedControllers: sets.New("none"),
		},
		{
			name: "policy-with-match-namespace-selector",
			policy: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "ValidatingPolicy",
    "metadata": {
        "name": "chech-labels"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        ""
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE",
                        "UPDATE"
                    ],
                    "resources": [
                        "pods"
                    ]
                }
            ],
            "namespaceSelector": {
                "matchLabels": {
                    "app": "nginx"
                }
            }
        },
        "variables": [
            {
                "name": "environment",
                "expression": "has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == 'prod'"
            }
        ],
        "validations": [
            {
                "expression": "variables.environment == true",
                "message": "Pod labels must be env=prod"
            }
        ]
    }
}`),
			expectedControllers: sets.New("none"),
		},
		{
			name: "policy-with-match-mixed-kinds-pod-podcontrollers",
			policy: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "ValidatingPolicy",
    "metadata": {
        "name": "chech-labels"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        ""
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE",
                        "UPDATE"
                    ],
                    "resources": [
                        "pods"
                    ]
                },
                {
                    "apiGroups": [
                        "apps"
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE",
                        "UPDATE"
                    ],
                    "resources": [
                        "deployments"
                    ]
                }
            ]
        },
        "variables": [
            {
                "name": "environment",
                "expression": "has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == 'prod'"
            }
        ],
        "validations": [
            {
                "expression": "variables.environment == true",
                "message": "Pod labels must be env=prod"
            }
        ]
    }
}`),
			expectedControllers: sets.New("none"),
		},
		{
			name: "policy-with-match-kinds-pod-only",
			policy: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "ValidatingPolicy",
    "metadata": {
        "name": "chech-labels"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        ""
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE",
                        "UPDATE"
                    ],
                    "resources": [
                        "pods"
                    ]
                }
            ]
        },
        "variables": [
            {
                "name": "environment",
                "expression": "has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == 'prod'"
            }
        ],
        "validations": [
            {
                "expression": "variables.environment == true",
                "message": "Pod labels must be env=prod"
            }
        ]
    }
}`),
			expectedControllers: podControllers,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var policy *policiesv1alpha1.ValidatingPolicy
			err := json.Unmarshal(test.policy, &policy)
			assert.NilError(t, err)

			applyAutoGen, controllers := canAutoGen(&policy.Spec)
			if !applyAutoGen {
				controllers = sets.New("none")
			}

			equalityTest := test.expectedControllers.Equal(controllers)
			assert.Assert(t, equalityTest, fmt.Sprintf("expected: %v, got: %v", test.expectedControllers, controllers))
		})
	}
}
