package autogen

import (
	"encoding/json"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestRuleName(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		index      int
		want       string
	}{
		{
			name:       "with identifier",
			identifier: "check-privileged",
			index:      3,
			want:       "autogen-check-privileged",
		},
		{
			name:       "without identifier falls back to numeric index",
			identifier: "",
			index:      0,
			want:       "autogen-validate-0",
		},
		{
			name:       "without identifier at a later index",
			identifier: "",
			index:      2,
			want:       "autogen-validate-2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RuleName(tt.identifier, tt.index))
		})
	}
}

func TestValidateUniqueIdentifiers(t *testing.T) {
	path := field.NewPath("spec").Child("validations")
	tests := []struct {
		name        string
		identifiers []string
		wantErrs    int
	}{
		{
			name:        "all validations have unique identifiers",
			identifiers: []string{"check-privileged", "check-run-as-non-root", "check-read-only-fs"},
			wantErrs:    0,
		},
		{
			name:        "no identifiers set, purely positional",
			identifiers: []string{"", "", ""},
			wantErrs:    0,
		},
		{
			name:        "mix of identifiers and empty entries",
			identifiers: []string{"check-privileged", "", "check-read-only-fs", ""},
			wantErrs:    0,
		},
		{
			name:        "duplicate identifier is rejected",
			identifiers: []string{"check-privileged", "check-run-as-non-root", "check-privileged"},
			wantErrs:    1,
		},
		{
			name:        "multiple duplicate identifiers are all reported",
			identifiers: []string{"a", "a", "a"},
			wantErrs:    2,
		},
		{
			name:        "empty identifiers never collide with each other",
			identifiers: []string{"", "check-privileged", ""},
			wantErrs:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateUniqueIdentifiers(path, tt.identifiers)
			assert.Len(t, errs, tt.wantErrs)
		})
	}
}

func TestIdentifiersFromAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        map[string]string
		wantErr     bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			want:        nil,
		},
		{
			name:        "annotation not present",
			annotations: map[string]string{"other": "value"},
			want:        nil,
		},
		{
			name:        "annotation present but empty",
			annotations: map[string]string{IdentifiersAnnotation: ""},
			want:        nil,
		},
		{
			name: "valid mapping",
			annotations: map[string]string{
				IdentifiersAnnotation: `{"object.spec.privileged == false":"check-privileged"}`,
			},
			want: map[string]string{"object.spec.privileged == false": "check-privileged"},
		},
		{
			name: "malformed json",
			annotations: map[string]string{
				IdentifiersAnnotation: `{not valid json`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IdentifiersFromAnnotations(tt.annotations)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
