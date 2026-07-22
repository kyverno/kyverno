package autogen

import (
	"encoding/json"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestGenerateRuleForControllers(t *testing.T) {
	tests := []struct {
		name          string
		controllers   sets.Set[string]
		policySpec    []byte
		generatedRule map[string]policiesv1beta1.ValidatingPolicyAutogen
	}{
		{
			name:        "autogen rule for deployments",
			controllers: sets.New("deployments"),
			policySpec: []byte(`{
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
				"validations": [
					{
						"expression": "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"
					}
				]
			}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenDefaults: {
					Targets: []policiesv1beta1.Target{
						{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{
											admissionregistrationv1.Create,
											admissionregistrationv1.Update,
										},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
							},
						},
					},
				},
			},
		},
		{
			name:        "autogen rule for deployments and daemonsets",
			controllers: sets.New("deployments", "daemonsets"),
			policySpec: []byte(`{
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
				"validations": [
					{
						"expression": "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"
					}
				]
			}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenDefaults: {
					Targets: []policiesv1beta1.Target{
						{Group: "apps", Version: "v1", Resource: "daemonsets", Kind: "DaemonSet"},
						{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{
											admissionregistrationv1.Create,
											admissionregistrationv1.Update,
										},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"daemonsets", "deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
							},
						},
					},
				},
			},
		},
		{
			name:        "autogen rule for deployments, daemonsets, statefulsets and replicasets",
			controllers: sets.New("deployments", "daemonsets", "statefulsets", "replicasets"),
			policySpec: []byte(`{
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
				"matchConditions": [
					{
						"name": "only for production",
						"expression": "has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'"
					}
				],
				"validations": [
					{
						"expression": "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"
					}
				]
			}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenDefaults: {
					Targets: []policiesv1beta1.Target{
						{Group: "apps", Version: "v1", Resource: "daemonsets", Kind: "DaemonSet"},
						{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
						{Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
						{Group: "apps", Version: "v1", Resource: "statefulsets", Kind: "StatefulSet"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{
											admissionregistrationv1.Create,
											admissionregistrationv1.Update,
										},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "only for production",
								Expression: "has(object.spec.template.metadata.labels) && has(object.spec.template.metadata.labels.prod) && object.spec.template.metadata.labels.prod == 'true'",
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
							},
						},
					},
				},
			},
		},
		{
			name:        "autogen preserves object.metadata.name in validations",
			controllers: sets.New("deployments"),
			policySpec: []byte(`{
				"matchConstraints": {
					"resourceRules": [{
						"apiGroups": [""],
						"apiVersions": ["v1"],
						"operations": ["CREATE", "UPDATE"],
						"resources": ["pods"]
					}]
				},
				"matchConditions": [{
					"name": "in allowed namespace",
					"expression": "resource.List('v1', 'configmaps', object.metadata.namespace).size() > 0"
				}],
				"validations": [{
					"expression": "object.metadata.name != ''"
				}]
			}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenDefaults: {
					Targets: []policiesv1beta1.Target{
						{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{
										admissionregistrationv1.Create,
										admissionregistrationv1.Update,
									},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"apps"},
										APIVersions: []string{"v1"},
										Resources:   []string{"deployments"},
									},
								},
							}},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{{
							Name:       "in allowed namespace",
							Expression: "resource.List('v1', 'configmaps', object.metadata.namespace).size() > 0",
						}},
						Validations: []admissionregistrationv1.Validation{{
							Expression: "object.metadata.name != ''",
						}},
					},
				},
			},
		},
		{
			name:        "autogen preserves object.metadata.namespace in match conditions",
			controllers: sets.New("deployments"),
			policySpec: []byte(`{
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
				"matchConditions": [
					{
						"name": "skip system namespaces",
						"expression": "!(object.metadata.namespace in ['opencost', 'kube-system']) && has(object.metadata.labels) && object.metadata.labels.prod == 'true'"
					}
				],
				"validations": [
					{
						"expression": "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"
					}
				]
			}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenDefaults: {
					Targets: []policiesv1beta1.Target{
						{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{
											admissionregistrationv1.Create,
											admissionregistrationv1.Update,
										},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "skip system namespaces",
								Expression: "!(object.metadata.namespace in ['opencost', 'kube-system']) && has(object.spec.template.metadata.labels) && object.spec.template.metadata.labels.prod == 'true'",
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var spec policiesv1beta1.ValidatingPolicySpec
			err := json.Unmarshal(test.policySpec, &spec)
			assert.NoError(t, err)
			genRule, err := generateRuleForControllers(spec, test.controllers)
			assert.NoError(t, err)
			assert.Equal(t, test.generatedRule, genRule)
		})
	}
}

func TestAutogenMetadataPathsSurviveJSONRoundTrip(t *testing.T) {
	spec := policiesv1beta1.ValidatingPolicySpec{
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				},
			}},
		},
		Variables: []admissionregistrationv1.Variable{{
			Name:       "workloadName",
			Expression: `object.metadata["name"]`,
		}},
		MatchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "workload metadata",
			Expression: `resource.Get('v1', 'configmaps', object.metadata.namespace, object.metadata.name) != null && oldObject.metadata.?annotations[?'example.com/key'].orValue('') == object.metadata.?annotations[?'example.com/key'].orValue('')`,
		}},
		Validations: []admissionregistrationv1.Validation{{
			Expression:        `object.metadata.?labels[?'app'].orValue('') == 'demo' && object.spec.containers.size() > 0`,
			MessageExpression: `object.metadata.name + ':' + oldObject.metadata.uid`,
		}},
		AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
			Key:             "workload",
			ValueExpression: `object.metadata.ownerReferences[0].uid + ':' + object.metadata.labels.app`,
		}},
	}

	generated, err := generateRuleForControllers(spec, sets.New("deployments", "cronjobs"))
	assert.NoError(t, err)

	deployment := generated[autogen.AutogenDefaults].Spec
	assert.Equal(t, `object.metadata["name"]`, deployment.Variables[0].Expression)
	assert.Equal(t, `resource.Get('v1', 'configmaps', object.metadata.namespace, object.metadata.name) != null && oldObject.spec.template.metadata.?annotations[?'example.com/key'].orValue('') == object.spec.template.metadata.?annotations[?'example.com/key'].orValue('')`, deployment.MatchConditions[0].Expression)
	assert.Equal(t, `object.spec.template.metadata.?labels[?'app'].orValue('') == 'demo' && object.spec.template.spec.containers.size() > 0`, deployment.Validations[0].Expression)
	assert.Equal(t, `object.metadata.name + ':' + oldObject.metadata.uid`, deployment.Validations[0].MessageExpression)
	assert.Equal(t, `object.metadata.ownerReferences[0].uid + ':' + object.spec.template.metadata.labels.app`, deployment.AuditAnnotations[0].ValueExpression)

	cronjob := generated[autogen.AutogenCronjobs].Spec
	assert.Equal(t, `object.spec.jobTemplate.spec.template.metadata.?labels[?'app'].orValue('') == 'demo' && object.spec.jobTemplate.spec.template.spec.containers.size() > 0`, cronjob.Validations[0].Expression)
	assert.Equal(t, `object.metadata.ownerReferences[0].uid + ':' + object.spec.jobTemplate.spec.template.metadata.labels.app`, cronjob.AuditAnnotations[0].ValueExpression)
}

func TestGenerateCronJobRule(t *testing.T) {
	tests := []struct {
		policySpec    []byte
		generatedRule map[string]policiesv1beta1.ValidatingPolicyAutogen
	}{
		{
			policySpec: []byte(`{
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
    "validations": [
        {
            "expression": "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"
        }
    ]
}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenCronjobs: {
					Targets: []policiesv1beta1.Target{
						{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{
											admissionregistrationv1.Create,
											admissionregistrationv1.Update,
										},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"batch"},
											APIVersions: []string{"v1"},
											Resources:   []string{"cronjobs"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
							},
						},
					},
				},
			},
		},
		{
			policySpec: []byte(`{
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
	"matchConditions": [
		{
	        "name": "only for production",
			"expression": "has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'"
	    }
	],
    "validations": [
        {
            "expression": "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"
        }
    ]
}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenCronjobs: {
					Targets: []policiesv1beta1.Target{
						{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{
											admissionregistrationv1.Create,
											admissionregistrationv1.Update,
										},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"batch"},
											APIVersions: []string{"v1"},
											Resources:   []string{"cronjobs"},
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "only for production",
								Expression: "has(object.spec.jobTemplate.spec.template.metadata.labels) && has(object.spec.jobTemplate.spec.template.metadata.labels.prod) && object.spec.jobTemplate.spec.template.metadata.labels.prod == 'true'",
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
							},
						},
					},
				},
			},
		},
		{
			policySpec: []byte(`{
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
            "message": "labels must be env=prod"
        }
    ]
}`),
			generatedRule: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				autogen.AutogenCronjobs: {
					Targets: []policiesv1beta1.Target{
						{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"},
					},
					Spec: &policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{
											admissionregistrationv1.Create,
											admissionregistrationv1.Update,
										},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"batch"},
											APIVersions: []string{"v1"},
											Resources:   []string{"cronjobs"},
										},
									},
								},
							},
						},
						Variables: []admissionregistrationv1.Variable{
							{
								Name:       "environment",
								Expression: "has(object.spec.jobTemplate.spec.template.metadata.labels) && 'env' in object.spec.jobTemplate.spec.template.metadata.labels && object.spec.jobTemplate.spec.template.metadata.labels['env'] == 'prod'",
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "variables.environment == true",
								Message:    "labels must be env=prod",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var spec policiesv1beta1.ValidatingPolicySpec
			err := json.Unmarshal(tt.policySpec, &spec)
			assert.NoError(t, err)
			genRule, err := generateRuleForControllers(spec, sets.New("cronjobs"))
			assert.NoError(t, err)
			assert.Equal(t, tt.generatedRule, genRule)
		})
	}
}
