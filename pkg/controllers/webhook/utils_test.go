package webhook

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

func Test_webhook_isEmpty(t *testing.T) {
	empty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	assert.Equal(t, empty.isEmpty(), true)
	notEmpty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	notEmpty.set("", "v1", "pods", "", admissionregistrationv1.NamespacedScope, kyvernov1.Create)
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
	assert.NoError(t, err)
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
		rules          sets.Set[ruleEntry]
		expectedResult []admissionregistrationv1.RuleWithOperations
	}{{
		rules: sets.New[ruleEntry](
			ruleEntry{"", "v1", "configmaps", "", admissionregistrationv1.NamespacedScope, kyvernov1.Create},
			ruleEntry{"", "v1", "pods", "", admissionregistrationv1.NamespacedScope, kyvernov1.Create},
		),
		expectedResult: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"configmaps", "pods", "pods/ephemeralcontainers"},
				Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
			},
		}},
	}, {
		rules: sets.New[ruleEntry](
			ruleEntry{"", "v1", "configmaps", "", admissionregistrationv1.NamespacedScope, kyvernov1.Create},
			ruleEntry{"", "v1", "pods", "", admissionregistrationv1.NamespacedScope, kyvernov1.Create},
			ruleEntry{"", "v1", "pods", "", admissionregistrationv1.NamespacedScope, kyvernov1.Update},
		),
		expectedResult: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"configmaps"},
				Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
			},
		}, {
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"pods", "pods/ephemeralcontainers"},
				Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
			},
		}},
	}}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			wh := &webhook{
				rules: testCase.rules,
			}
			result := wh.buildRulesWithOperations()
			assert.Equal(t, testCase.expectedResult, result)
		})
	}
}

func Test_less(t *testing.T) {
	tests := []struct {
		name string
		do   func() int
		want int
	}{{
		do: func() int {
			return less([]int{0}, []int{0, 0})
		},
		want: -1,
	}, {
		do: func() int {
			return less([]int{0, 0}, []int{0})
		},
		want: 1,
	}, {
		do: func() int {
			return less([]int{0}, []int{1})
		},
		want: -1,
	}, {
		do: func() int {
			return less([]int{1}, []int{0})
		},
		want: 1,
	}, {
		do: func() int {
			return less([]int{0, 0}, []int{0, 0})
		},
		want: 0,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.do()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_collectResourceDescriptions(t *testing.T) {
	tests := []struct {
		name       string
		rule       kyvernov1.Rule
		defaultOps []kyvernov1.AdmissionOperation
		want       webhookConfig
	}{{
		name:       "empty",
		rule:       kyvernov1.Rule{},
		defaultOps: allOperations,
		want:       webhookConfig{},
	}, {
		name: "match any - default ops",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(kyvernov1.Create, kyvernov1.Update),
		},
	}, {
		name: "match any - ops",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
					},
				}},
			},
		},
		defaultOps: allOperations,
		want: webhookConfig{
			"ConfigMap": sets.New(kyvernov1.Create, kyvernov1.Update),
		},
	}, {
		name: "match any - multiple",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Create},
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{
							"Secret",
						},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
		},
		defaultOps: allOperations,
		want: webhookConfig{
			"ConfigMap": sets.New(kyvernov1.Create),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "match all - default ops",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				All: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(kyvernov1.Create, kyvernov1.Update),
		},
	}, {
		name: "match any - ops",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				All: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
					},
				}},
			},
		},
		defaultOps: allOperations,
		want: webhookConfig{
			"ConfigMap": sets.New(kyvernov1.Create, kyvernov1.Update),
		},
	}, {
		name: "match all - multiple",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Create},
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
		},
		defaultOps: allOperations,
		want: webhookConfig{
			"ConfigMap": sets.New(kyvernov1.Create),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - no ops",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New[kyvernov1.AdmissionOperation](),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - ops",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Connect},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(kyvernov1.Create, kyvernov1.Update, kyvernov1.Delete),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - with annotations",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
						Annotations: map[string]string{
							"foo": "bar",
						},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Connect},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(allOperations...),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - with name",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Name:       "foo",
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Connect},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(allOperations...),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - with names",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Names:      []string{"foo"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Connect},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(allOperations...),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - with namespaces",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Namespaces: []string{"foo"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Connect},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(allOperations...),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - with selector",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Connect},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(allOperations...),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}, {
		name: "exclude - with ns selector",
		rule: kyvernov1.Rule{
			MatchResources: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"ConfigMap"},
						Operations: allOperations,
					},
				}, {
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
					},
				}},
			},
			ExcludeResources: &kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
						Operations: []kyvernov1.AdmissionOperation{kyvernov1.Connect},
					},
				}},
			},
		},
		defaultOps: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
		want: webhookConfig{
			"ConfigMap": sets.New(allOperations...),
			"Secret":    sets.New(kyvernov1.Update),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectResourceDescriptions(tt.rule, tt.defaultOps...)
			assert.Equal(t, tt.want, got)
		})
	}
}
