package webhook

import (
	"encoding/json"
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"gotest.tools/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

func Test_webhook_isEmpty(t *testing.T) {
	empty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	assert.Equal(t, empty.isEmpty(), true)
	notEmpty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	notEmpty.set(GroupVersionResourceScopeOperation{
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Scope:                admissionregistrationv1.NamespacedScope,
	})
	assert.Equal(t, notEmpty.isEmpty(), false)
}

var policy = `
{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "disallow-unsigned-images"
    },
    "spec": {
        "background": false,
        "rules": [
            {
                "name": "replace-image-registry",
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
                    "foreach": [
                        {
                            "list": "request.object.spec.containers",
                            "patchStrategicMerge": {
                                "spec": {
                                    "containers": [
                                        {
                                            "name": "{{ element.name }}",
                                            "image": "{{ regex_replace_all_literal('.*(.*)/', '{{element.image}}', 'pratikrshah/' )}}"
                                        }
                                    ]
                                }
                            }
                        }
                    ]
                }
            },
            {
                "name": "disallow-unsigned-images-rule",
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
                "verifyImages": [
                    {
                        "imageReferences": [
                            "*"
                        ],
                        "verifyDigest": false,
                        "required": null,
                        "mutateDigest": false,
                        "attestors": [
                            {
                                "count": 1,
                                "entries": [
                                    {
                                        "keys": {
                                            "publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEHsra9WSDxt9qv84KF4McNVCGjMFq\ne96mWCQxGimL9Ltj6F3iXmlo8sUalKfJ7SBXpy8hwyBfXBBAmCalsp5xEw==\n-----END PUBLIC KEY-----"
                                        }
                                    }
                                ]
                            }
                        ]
                    }
                ]
            },
            {
                "name": "check-image",
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
                "context": [
                    {
                        "name": "keys",
                        "configMap": {
                            "name": "keys",
                            "namespace": "default"
                        }
                    }
                ],
                "verifyImages": [
                    {
                        "imageReferences": [
                            "ghcr.io/myorg/myimage*"
                        ],
                        "required": true,
                        "attestors": [
                            {
                                "count": 1,
                                "entries": [
                                    {
                                        "keys": {
                                            "publicKeys": "{{ keys.data.production }}"
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
}
`

func Test_RuleCount(t *testing.T) {
	var cpol kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(policy), &cpol)
	assert.NilError(t, err)
	status := cpol.GetStatus()
	rules := autogen.ComputeRules(&cpol, "")
	setRuleCount(rules, status)
	assert.Equal(t, status.RuleCount.Validate, 0)
	assert.Equal(t, status.RuleCount.Generate, 0)
	assert.Equal(t, status.RuleCount.Mutate, 1)
	assert.Equal(t, status.RuleCount.VerifyImages, 2)
}

func TestBuildRulesWithOperations(t *testing.T) {
	testCases := []struct {
		name           string
		rules          sets.Set[GroupVersionResourceScopeOperation]
		expectedResult []admissionregistrationv1.RuleWithOperations
	}{
		{
			name: "Test Case 1",
			rules: sets.New[GroupVersionResourceScopeOperation](
				GroupVersionResourceScopeOperation{
					GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
					Scope:                admissionregistrationv1.NamespacedScope,
					Operation:            webhookCreate,
				},
				GroupVersionResourceScopeOperation{
					GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Scope:                admissionregistrationv1.NamespacedScope,
					Operation:            webhookCreate,
				},
				GroupVersionResourceScopeOperation{
					GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Scope:                admissionregistrationv1.NamespacedScope,
					Operation:            webhookUpdate,
				},
			),
			expectedResult: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{webhookCreate},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
						Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
					},
				}, {
					Operations: []admissionregistrationv1.OperationType{webhookCreate, webhookUpdate},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods", "pods/ephemeralcontainers"},
						Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			wh := &webhook{
				rules: testCase.rules,
			}

			result := wh.buildRules()
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}
