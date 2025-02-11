package autogen

import (
	"encoding/json"
	"reflect"
	"testing"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"gotest.tools/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestGenerateRuleForControllers(t *testing.T) {
	tests := []struct {
		name          string
		controllers   string
		policySpec    []byte
		generatedRule kyvernov2alpha1.AutogenRule
	}{
		{
			name:        "autogen rule for deployments",
			controllers: "deployments",
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
			generatedRule: kyvernov2alpha1.AutogenRule{
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
		{
			name:        "autogen rule for deployments and daemonsets",
			controllers: "deployments,daemonsets",
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
			generatedRule: kyvernov2alpha1.AutogenRule{
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
									Resources:   []string{"deployments", "daemonsets"},
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
		{
			name:        "autogen rule for deployments, daemonsets, statefulsets and replicasets",
			controllers: "deployments,daemonsets,statefulsets,replicasets",
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
			generatedRule: kyvernov2alpha1.AutogenRule{
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
									Resources:   []string{"deployments", "daemonsets", "statefulsets", "replicasets"},
								},
							},
						},
					},
				},
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "autogen-only for production",
						Expression: "!(object.Kind =='Deployment' || object.Kind =='ReplicaSet' || object.Kind =='StatefulSet' || object.Kind =='DaemonSet') || has(object.spec.template.metadata.labels) && has(object.spec.template.metadata.labels.prod) && object.spec.template.metadata.labels.prod == 'true'",
					},
				},
				Validations: []admissionregistrationv1.Validation{
					{
						Expression: "object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var spec *kyvernov2alpha1.ValidatingPolicySpec
			err := json.Unmarshal(test.policySpec, &spec)
			assert.NilError(t, err)

			genRule, err := generateRuleForControllers(spec, test.controllers)
			assert.NilError(t, err)

			if !reflect.DeepEqual(genRule, &test.generatedRule) {
				t.Errorf("generateRuleForControllers() = %v, want %v", genRule, test.generatedRule)
			}
		})
	}
}

func TestGenerateCronJobRule(t *testing.T) {
	tests := []struct {
		policySpec    []byte
		generatedRule kyvernov2alpha1.AutogenRule
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
			generatedRule: kyvernov2alpha1.AutogenRule{
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
			generatedRule: kyvernov2alpha1.AutogenRule{
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
						Name:       "autogen-cronjobs-only for production",
						Expression: "!(object.Kind =='CronJob') || has(object.spec.jobTemplate.spec.template.metadata.labels) && has(object.spec.jobTemplate.spec.template.metadata.labels.prod) && object.spec.jobTemplate.spec.template.metadata.labels.prod == 'true'",
					},
				},
				Validations: []admissionregistrationv1.Validation{
					{
						Expression: "object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
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
			generatedRule: kyvernov2alpha1.AutogenRule{
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
	}
	for i, tt := range tests {
		var spec *kyvernov2alpha1.ValidatingPolicySpec
		err := json.Unmarshal(tt.policySpec, &spec)
		assert.NilError(t, err)

		genRule, err := generateCronJobRule(spec, "cronjobs")
		assert.NilError(t, err)

		if !reflect.DeepEqual(genRule, &tt.generatedRule) {
			t.Errorf("%v: generateCronJobRule() = %v, want %v", i, genRule, tt.generatedRule)
		}
	}
}

func TestUpdateGenRuleByte(t *testing.T) {
	tests := []struct {
		pbyte    []byte
		resource string
		want     []byte
	}{
		{
			pbyte:    []byte("object.spec"),
			resource: "pods",
			want:     []byte("object.spec.template.spec"),
		},
		{
			pbyte:    []byte("oldObject.spec"),
			resource: "pods",
			want:     []byte("oldObject.spec.template.spec"),
		},
		{
			pbyte:    []byte("object.spec"),
			resource: "cronjobs",
			want:     []byte("object.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte:    []byte("oldObject.spec"),
			resource: "cronjobs",
			want:     []byte("oldObject.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte:    []byte("object.metadata"),
			resource: "pods",
			want:     []byte("object.spec.template.metadata"),
		},
		{
			pbyte:    []byte("object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
			resource: "cronjobs",
			want:     []byte("object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
		},
		{
			pbyte:    []byte("object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
			resource: "pods",
			want:     []byte("object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
		},
	}
	for _, tt := range tests {
		got := updateFields(tt.pbyte, tt.resource)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("updateGenRuleByte() = %v, want %v", string(got), string(tt.want))
		}
	}
}
