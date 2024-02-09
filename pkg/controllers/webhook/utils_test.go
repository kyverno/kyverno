package webhook

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"gotest.tools/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_webhook_isEmpty(t *testing.T) {
	empty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	assert.Equal(t, empty.isEmpty(), true)
	notEmpty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	notEmpty.set(GroupVersionResourceScope{
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
        "validationFailureAction": "enforce",
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
	var cpol kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policy), &cpol)
	assert.NilError(t, err)
	status := cpol.GetStatus()
	rules := autogen.ComputeRules(&cpol)
	setRuleCount(rules, status)
	assert.Equal(t, status.RuleCount.Validate, 0)
	assert.Equal(t, status.RuleCount.Generate, 0)
	assert.Equal(t, status.RuleCount.Mutate, 1)
	assert.Equal(t, status.RuleCount.VerifyImages, 2)
}

func TestGetMinimumOperations(t *testing.T) {
	testCases := []struct {
		name           string
		inputMap       map[string]bool
		expectedResult []admissionregistrationv1.OperationType
	}{
		{
			name: "Test Case 1",
			inputMap: map[string]bool{
				webhookCreate: true,
				webhookUpdate: false,
				webhookDelete: true,
			},
			expectedResult: []admissionregistrationv1.OperationType{webhookCreate, webhookDelete},
		},
		{
			name: "Test Case 2",
			inputMap: map[string]bool{
				webhookCreate:  false,
				webhookUpdate:  false,
				webhookDelete:  false,
				webhookConnect: true,
			},
			expectedResult: []admissionregistrationv1.OperationType{webhookConnect},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := getMinimumOperations(testCase.inputMap)
			sort.Slice(result, func(i, j int) bool {
				return result[i] < result[j]
			})
			sort.Slice(testCase.expectedResult, func(i, j int) bool {
				return testCase.expectedResult[i] < testCase.expectedResult[j]
			})

			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}

func TestComputeOperationsForMutatingWebhookConf(t *testing.T) {
	testCases := []struct {
		name           string
		rules          []kyverno.Rule
		expectedResult map[string]bool
	}{
		{
			name: "Test Case 1",
			rules: []kyverno.Rule{
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []v1.AdmissionOperation{webhookCreate},
						},
					},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate: true,
			},
		},
		{
			name: "Test Case 2",
			rules: []kyverno.Rule{
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources:   kyverno.MatchResources{},
					ExcludeResources: kyverno.MatchResources{},
				},
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources:   kyverno.MatchResources{},
					ExcludeResources: kyverno.MatchResources{},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate: true,
				webhookUpdate: true,
			},
		},
		{
			name: "Test Case 2",
			rules: []kyverno.Rule{
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{},
					ExcludeResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []v1.AdmissionOperation{webhookCreate},
						},
					},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate: false,
				webhookUpdate: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string]bool
			for _, r := range testCase.rules {
				result = computeOperationsForMutatingWebhookConf(r, make(map[string]bool))
			}
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}

func TestComputeOperationsForValidatingWebhookConf(t *testing.T) {
	testCases := []struct {
		name           string
		rules          []kyverno.Rule
		expectedResult map[string]bool
	}{
		{
			name: "Test Case 1",
			rules: []kyverno.Rule{
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []v1.AdmissionOperation{webhookCreate},
						},
					},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate: true,
			},
		},
		{
			name: "Test Case 2",
			rules: []kyverno.Rule{
				{
					MatchResources:   kyverno.MatchResources{},
					ExcludeResources: kyverno.MatchResources{},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate:  true,
				webhookUpdate:  true,
				webhookConnect: true,
				webhookDelete:  true,
			},
		},
		{
			name: "Test Case 3",
			rules: []kyverno.Rule{
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []v1.AdmissionOperation{webhookCreate, webhookUpdate},
						},
					},
					ExcludeResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []v1.AdmissionOperation{webhookDelete},
						},
					},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate: true,
				webhookUpdate: true,
				webhookDelete: false,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string]bool
			for _, r := range testCase.rules {
				result = computeOperationsForValidatingWebhookConf(r, make(map[string]bool))
			}
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}
