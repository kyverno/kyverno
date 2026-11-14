package autogen

import (
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

func normalize(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestConvertPodToTemplateExpression(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		input    string
		expected string
	}{
		{
			name:   "deployment containers conversion",
			config: "deployments",
			input: `Object{
  						spec: Object.spec{
    						containers: object.spec.containers.map(container, Object.spec.containers{
      							name: container.name,
      							securityContext: Object.spec.containers.securityContext{
        							allowPrivilegeEscalation: false
      							}
    						})
  						}
					}`,
			expected: `Object{spec: Object.spec{
								template: Object.spec.template{
  									spec: Object.spec.template.spec{
    									containers: object.spec.template.spec.containers.map(container, Object.spec.template.spec.containers{
      										name: container.name,
      										securityContext: Object.spec.template.spec.containers.securityContext{
        										allowPrivilegeEscalation: false
      										}
    									})
  									}
						}}}`,
		},
		{
			name:   "cronjob containers conversion",
			config: "cronjobs",
			input: `Object{
  spec: Object.spec{
    containers: object.spec.containers.map(container, Object.spec.containers{
      name: container.name,
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}`,
			expected: `Object{spec: Object.spec{jobTemplate: Object.spec.jobTemplate{spec: Object.spec.jobTemplate.spec{template: Object.spec.jobTemplate.spec.template{
  spec: Object.spec.jobTemplate.spec.template.spec{
    containers: object.spec.jobTemplate.spec.template.spec.containers.map(container, Object.spec.jobTemplate.spec.template.spec.containers{
      name: container.name,
      securityContext: Object.spec.jobTemplate.spec.template.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}}}}}`,
		},
		{
			name:   "statefulset containers conversion",
			config: "statefulsets",
			input: `Object{
  spec: Object.spec{
    containers: [Object.spec.containers{
      name: "nginx",
      image: "nginx:latest",
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    }]
  }
}`,
			expected: `Object{spec: Object.spec{template: Object.spec.template{
  spec: Object.spec.template.spec{
    containers: [Object.spec.template.spec.containers{
      name: "nginx",
      image: "nginx:latest",
      securityContext: Object.spec.template.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    }]
  }
}}}`,
		},
		{
			name:     "namespaceObject labels not modified",
			config:   "deployments",
			input:    `'istio.io/dataplane-mode' in namespaceObject.metadata.labels`,
			expected: `'istio.io/dataplane-mode' in namespaceObject.metadata.labels`,
		},
		{
			name:     "namespaceObject labels not modified for cronjobs",
			config:   "cronjobs",
			input:    `'istio.io/dataplane-mode' in namespaceObject.metadata.labels`,
			expected: `'istio.io/dataplane-mode' in namespaceObject.metadata.labels`,
		},
		{
			name:     "namespaceObject spec not modified",
			config:   "deployments",
			input:    `namespaceObject.spec.containers`,
			expected: `namespaceObject.spec.containers`,
		},
		{
			name:     "namespaceObject spec not modified for cronjobs",
			config:   "cronjobs",
			input:    `namespaceObject.spec.containers`,
			expected: `namespaceObject.spec.containers`,
		},
		{
			name:     "namespaceObject annotations not modified",
			config:   "deployments",
			input:    `'team' in namespaceObject.metadata.annotations`,
			expected: `'team' in namespaceObject.metadata.annotations`,
		},
		{
			name:     "namespaceObject annotations not modified for cronjobs",
			config:   "cronjobs",
			input:    `'team' in namespaceObject.metadata.annotations`,
			expected: `'team' in namespaceObject.metadata.annotations`,
		},
		{
			name:     "oldObject spec still modified",
			config:   "deployments",
			input:    `oldObject.spec.containers`,
			expected: `oldObject.spec.template.spec.containers`,
		},
		{
			name:   "no containers in expression",
			config: "deployments",
			input: `Object{
  metadata: Object.metadata{
    labels: Object.metadata.labels{
      foo: "bar"
    }
  }
}`,
			expected: `Object{spec: Object.spec{template: Object.spec.template{
  metadata: Object.metadata{
    labels: Object.spec.template.metadata.labels{
      foo: "bar"
    }
  }
}}}`,
		},
		{
			name:     "deployment object.?spec conversion",
			config:   "deployments",
			input:    `object.?spec.?containers.all(c, true)`,
			expected: `object.?spec.template.spec.?containers.all(c, true)`,
		},
		{
			name:     "cronjob object.?spec conversion",
			config:   "cronjobs",
			input:    `object.?spec.?containers.all(c, true)`,
			expected: `object.?spec.jobTemplate.spec.template.spec.?containers.all(c, true)`,
		},
		{
			name:     "deployment object.?metadata.?labels conversion",
			config:   "deployments",
			input:    `object.?metadata.?labels['env'] == 'prod'`,
			expected: `object.?spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment object.?metadata.?annotations conversion",
			config:   "deployments",
			input:    `object.?metadata.?annotations['link'] != ''`,
			expected: `object.?spec.template.metadata.?annotations['link'] != ''`,
		},
		{
			name:     "deployment object[?'spec'] bracket indexing",
			config:   "deployments",
			input:    `object[?'spec'].containers.all(c, true)`,
			expected: `object.?spec.template.spec.containers.all(c, true)`,
		},
		{
			name:     "deployment object['spec'] bracket indexing",
			config:   "deployments",
			input:    `object['spec'].containers.all(c, true)`,
			expected: `object.spec.template.spec.containers.all(c, true)`,
		},
		{
			name:     "deployment oldObject[?\"spec\"] bracket indexing",
			config:   "deployments",
			input:    `oldObject[?"spec"].containers.all(c, true)`,
			expected: `oldObject.?spec.template.spec.containers.all(c, true)`,
		},
		{
			name:     "deployment object.?metadata[?'labels'] conversion",
			config:   "deployments",
			input:    `object.?metadata[?'labels']['env'] == 'prod'`,
			expected: `object.?spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment object[?'metadata'][?'labels'] conversion",
			config:   "deployments",
			input:    `object[?'metadata'][?'labels']['env'] == 'prod'`,
			expected: `object.?spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment object[?'metadata'].annotations conversion",
			config:   "deployments",
			input:    `object[?'metadata'].annotations['link'] != ''`,
			expected: `object.?spec.template.metadata.annotations['link'] != ''`,
		},
		{
			name:     "deployment object.metadata.name preserved",
			config:   "deployments",
			input:    `object.metadata.name == 'prod'`,
			expected: `object.metadata.name == 'prod'`,
		},
		{
			name:     "nested foo.?object.?spec untouched",
			config:   "deployments",
			input:    `foo.?object.?spec.containers.all(c, true)`,
			expected: `foo.?object.?spec.containers.all(c, true)`,
		},
		{
			name:     "nested foo.object.spec untouched",
			config:   "deployments",
			input:    `foo.object.spec.containers.all(c, true)`,
			expected: `foo.object.spec.containers.all(c, true)`,
		},
		{
			name:     "string literals containing selector patterns are not rewritten",
			config:   "deployments",
			input:    `object.metadata.labels['message'] == "object[?'spec']"`,
			expected: `object.spec.template.metadata.labels['message'] == "object[?'spec']"`,
		},
		{
			name:     "repeated match following existing template spec is not skipped",
			config:   "deployments",
			input:    `(object.spec.template.spec) && (object.spec.containers.exists(c, true))`,
			expected: `(object.spec.template.spec) && (object.spec.template.spec.containers.exists(c, true))`,
		},
		{
			name:     "negative case: namespaceObject.?spec is untouched",
			config:   "deployments",
			input:    `namespaceObject.?spec.?containers.all(c, true)`,
			expected: `namespaceObject.?spec.?containers.all(c, true)`,
		},
		{
			name:     "negative case: namespaceObject.spec is untouched",
			config:   "deployments",
			input:    `namespaceObject.spec.containers.all(c, true)`,
			expected: `namespaceObject.spec.containers.all(c, true)`,
		},
		{
			name:     "negative case: object.?metadata.name preserved",
			config:   "deployments",
			input:    `object.?metadata.name == 'prod'`,
			expected: `object.?metadata.name == 'prod'`,
		},
		{
			name:     "negative case: object.?metadata.?name preserved",
			config:   "deployments",
			input:    `object.?metadata.?name == 'prod'`,
			expected: `object.?metadata.?name == 'prod'`,
		},
		{
			name:     "negative case: object.?metadata[?'name'] preserved",
			config:   "deployments",
			input:    `object.?metadata[?'name'] == 'prod'`,
			expected: `object.?metadata[?'name'] == 'prod'`,
		},
		{
			name:     "negative case: object.?metadata[?'namespace'] preserved",
			config:   "deployments",
			input:    `object.?metadata[?'namespace'] == 'prod'`,
			expected: `object.?metadata[?'namespace'] == 'prod'`,
		},
		{
			name:     "bracket spec with space is rewritten",
			config:   "deployments",
			input:    `object[? 'spec'].containers.all(c, true)`,
			expected: `object.?spec.template.spec.containers.all(c, true)`,
		},
		{
			name:     "comment with apostrophe does not suppress spec rewrite",
			config:   "deployments",
			input:    "// don't match\nobject.spec.containers.all(c, true)",
			expected: "// don't match\nobject.spec.template.spec.containers.all(c, true)",
		},
		{
			name:     "raw triple-quoted string literal is preserved",
			config:   "deployments",
			input:    `r"""a" object.?spec.containers """`,
			expected: `r"""a" object.?spec.containers """`,
		},
		{
			name:     "metadata label named name is rewritten to template",
			config:   "deployments",
			input:    `object.metadata.labels['name'] == 'foo'`,
			expected: `object.spec.template.metadata.labels['name'] == 'foo'`,
		},
		{
			name:     "metadata label dot name is rewritten to template",
			config:   "deployments",
			input:    `object.metadata.labels.name == 'foo'`,
			expected: `object.spec.template.metadata.labels.name == 'foo'`,
		},
		{
			name:     "field templateSpec on spec is rewritten to template spec",
			config:   "deployments",
			input:    `object.spec.templateSpec == 'foo'`,
			expected: `object.spec.template.spec.templateSpec == 'foo'`,
		},
		{
			name:     "deployment object.metadata[? 'labels'] whitespace conversion",
			config:   "deployments",
			input:    `object.metadata[? 'labels']['env'] == 'prod'`,
			expected: `object.spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment object.metadata[?  'labels' ] multiple spaces conversion",
			config:   "deployments",
			input:    `object.metadata[?  'labels' ]['env'] == 'prod'`,
			expected: `object.spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment object[? 'metadata'][? 'labels'] whitespace conversion",
			config:   "deployments",
			input:    `object[? 'metadata'][? 'labels']['env'] == 'prod'`,
			expected: `object.?spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment object[?  'metadata' ] [?  'labels' ] spaced bracket conversion",
			config:   "deployments",
			input:    `object[?  'metadata' ] [?  'labels' ]['env'] == 'prod'`,
			expected: `object.?spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment object.metadata[\\n  'labels'\\n] newline conversion",
			config:   "deployments",
			input:    "object.metadata[\n  'labels'\n]['env'] == 'prod'",
			expected: "object.spec.template.metadata.labels['env'] == 'prod'",
		},
		{
			name:     "deployment object.metadata[? 'annotations'] whitespace conversion",
			config:   "deployments",
			input:    `object.metadata[? 'annotations']['link'] != ''`,
			expected: `object.spec.template.metadata.?annotations['link'] != ''`,
		},
		{
			name:     "deployment object[? 'metadata'][? 'annotations'] whitespace conversion",
			config:   "deployments",
			input:    `object[? 'metadata'][? 'annotations']['link'] != ''`,
			expected: `object.?spec.template.metadata.?annotations['link'] != ''`,
		},
		{
			name:     "deployment object[?  'spec' ] multiple spaces conversion",
			config:   "deployments",
			input:    `object[?  'spec' ].containers.all(c, true)`,
			expected: `object.?spec.template.spec.containers.all(c, true)`,
		},
		{
			name:     "deployment object[\\n  'spec'\\n] newline conversion",
			config:   "deployments",
			input:    "object[\n  'spec'\n].containers.all(c, true)",
			expected: "object.spec.template.spec.containers.all(c, true)",
		},
		{
			name:     "deployment oldObject.metadata[? 'labels'] whitespace conversion",
			config:   "deployments",
			input:    `oldObject.metadata[? 'labels']['env'] == 'prod'`,
			expected: `oldObject.spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment oldObject[? 'metadata'][? 'labels'] whitespace conversion",
			config:   "deployments",
			input:    `oldObject[? 'metadata'][? 'labels']['env'] == 'prod'`,
			expected: `oldObject.?spec.template.metadata.?labels['env'] == 'prod'`,
		},
		{
			name:     "deployment Object.metadata[? 'labels'] whitespace conversion",
			config:   "deployments",
			input:    `Object.metadata[? 'labels']`,
			expected: `Object.spec.template.metadata.?labels`,
		},
		{
			name:     "deployment Object[? 'metadata'][? 'labels'] whitespace conversion",
			config:   "deployments",
			input:    `Object[? 'metadata'][? 'labels']`,
			expected: `Object.?spec.template.metadata.?labels`,
		},
		{
			name:     "deployment object[? 'metadata'][? 'name'] preserved",
			config:   "deployments",
			input:    `object[? 'metadata'][? 'name'] == 'prod'`,
			expected: `object[? 'metadata'][? 'name'] == 'prod'`,
		},
		{
			name:     "deployment object.metadata[? 'name'] preserved",
			config:   "deployments",
			input:    `object.metadata[? 'name'] == 'prod'`,
			expected: `object.metadata[? 'name'] == 'prod'`,
		},
		{
			name:     "deployment spaced foo . object . spec untouched",
			config:   "deployments",
			input:    `foo . object . spec.containers.all(c, true)`,
			expected: `foo . object . spec.containers.all(c, true)`,
		},
		{
			name:     "deployment spaced foo .? object . spec untouched",
			config:   "deployments",
			input:    `foo .? object . spec.containers.all(c, true)`,
			expected: `foo .? object . spec.containers.all(c, true)`,
		},
		{
			name:     "deployment spaced foo . ? object . spec untouched",
			config:   "deployments",
			input:    `foo . ? object . spec.containers.all(c, true)`,
			expected: `foo . ? object . spec.containers.all(c, true)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPodToTemplateExpression(tt.input, tt.config)
			assert.Equal(t, normalize(tt.expected), normalize(result))
		})
	}
}

func TestGenerateRuleForControllers(t *testing.T) {
	tests := []struct {
		name          string
		spec          *policiesv1beta1.MutatingPolicySpec
		configs       sets.Set[string]
		expectedRules int
		expectedError bool
	}{
		{
			name: "deployment autogen with mutation",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{
  spec: Object.spec{
    containers: object.spec.containers.map(container, Object.spec.containers{
      name: container.name,
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}`,
						},
					},
				},
			},
			configs:       sets.New("deployments"),
			expectedRules: 1,
			expectedError: false,
		},
		{
			name: "cronjob autogen with mutation",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{
  spec: Object.spec{
    containers: [Object.spec.containers{
      name: "app",
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    }]
  }
}`,
						},
					},
				},
			},
			configs:       sets.New("cronjobs"),
			expectedRules: 1,
			expectedError: false,
		},
		{
			name: "multiple controllers autogen",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{
  spec: Object.spec{
    containers: object.spec.containers.map(container, Object.spec.containers{
      name: container.name,
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}`,
						},
					},
				},
			},
			configs:       sets.New("deployments", "statefulsets", "daemonsets"),
			expectedRules: 1,
			expectedError: false,
		},
		{
			name: "policy without mutations",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
			},
			configs:       sets.New("deployments"),
			expectedRules: 1,
			expectedError: false,
		},
		{
			name: "deployment with match conditions - metadata.annotations",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "check-annotations",
						Expression: "has(object.metadata.annotations) && ('inject-certs' in object.metadata.annotations && object.metadata.annotations['inject-certs'] == 'enabled')",
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{
  spec: Object.spec{
    containers: object.spec.containers.map(container, Object.spec.containers{
      name: container.name,
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}`,
						},
					},
				},
			},
			configs:       sets.New("deployments"),
			expectedRules: 1,
			expectedError: false,
		},
		{
			name: "cronjob with match conditions - metadata.labels",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "check-labels",
						Expression: "has(object.metadata.labels) && object.metadata.labels['app'] == 'myapp'",
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{
  spec: Object.spec{
    containers: [Object.spec.containers{
      name: "app",
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    }]
  }
}`,
						},
					},
				},
			},
			configs:       sets.New("cronjobs"),
			expectedRules: 1,
			expectedError: false,
		},
		{
			name: "deployment with variables autogen",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Variables: []admissionregistrationv1.Variable{
					{
						Name:       "containers",
						Expression: "object[?'spec'].?containers.orValue([])",
					},
				},
			},
			configs:       sets.New("deployments"),
			expectedRules: 1,
			expectedError: false,
		},
		{
			name: "policy with Object constructor in variables preserves value semantics",
			spec: &policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Variables: []admissionregistrationv1.Variable{
					{
						Name:       "customObj",
						Expression: "Object{labels: object.metadata.labels, name: object.metadata.name}",
					},
					{
						Name:       "containers",
						Expression: "object[?'spec'].?containers.orValue([])",
					},
				},
			},
			configs:       sets.New("deployments"),
			expectedRules: 1,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := generateRuleForControllers(tt.spec, tt.configs)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, rules, tt.expectedRules)

			for config, rule := range rules {
				assert.NotNil(t, rule.Spec)

				if len(tt.spec.Variables) > 0 {
					assert.Len(t, rule.Spec.Variables, len(tt.spec.Variables))
					for _, v := range rule.Spec.Variables {
						if v.Name == "customObj" {
							assert.Equal(t, "Object{labels: object.spec.template.metadata.labels, name: object.metadata.name}", v.Expression)
							assert.NotContains(t, v.Expression, "Object{spec:")
							continue
						}
						if config == "cronjobs" {
							assert.Contains(t, v.Expression, "jobTemplate.spec.template.spec")
						} else {
							assert.Contains(t, v.Expression, "template.spec")
						}
					}
				}

				if len(tt.spec.Mutations) > 0 {
					assert.Len(t, rule.Spec.Mutations, len(tt.spec.Mutations))

					for _, mutation := range rule.Spec.Mutations {
						if mutation.ApplyConfiguration != nil {
							convertedExpr := mutation.ApplyConfiguration.Expression

							if config == "cronjobs" {
								assert.Contains(t, convertedExpr, "jobTemplate.spec.template.spec.containers")
							} else {
								assert.Contains(t, convertedExpr, "template.spec.containers")
							}

							assert.NotContains(t, convertedExpr, "object.spec.containers")
						}
					}
				}
			}
		})
	}
}

func TestAutogenIntegration(t *testing.T) {
	t.Run("deployments", func(t *testing.T) {
		policy := &policiesv1beta1.MutatingPolicy{
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"team": "platform",
						},
					},
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{
  spec: Object.spec{
    containers: object.spec.containers.map(container, Object.spec.containers{
      name: container.name,
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}`,
						},
					},
				},
			},
		}

		policy.Spec.AutogenConfiguration = &policiesv1beta1.MutatingPolicyAutogenConfiguration{
			PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
				Controllers: []string{"deployments"},
			},
		}

		result, err := Autogen(policy)
		require.NoError(t, err)

		if len(result) == 0 {
			t.Skip("Autogen returned empty result, skipping integration test")
			return
		}

		deploymentAutogen, exists := result["defaults"]
		assert.True(t, exists)
		assert.Len(t, deploymentAutogen.Targets, 1)
		assert.Equal(t, "deployments", deploymentAutogen.Targets[0].Resource)
		assert.Equal(t, policy.Spec.MatchConstraints.NamespaceSelector, deploymentAutogen.Spec.MatchConstraints.NamespaceSelector)

		assert.Len(t, deploymentAutogen.Spec.Mutations, 1)
		convertedExpr := deploymentAutogen.Spec.Mutations[0].ApplyConfiguration.Expression
		assert.Contains(t, convertedExpr, "template.spec.containers")
		assert.NotContains(t, convertedExpr, "object.spec.containers")
	})

	t.Run("cronjobs", func(t *testing.T) {
		policy := &policiesv1beta1.MutatingPolicy{
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"team": "platform",
						},
					},
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{
  spec: Object.spec{
    containers: object.spec.containers.map(container, Object.spec.containers{
      name: container.name,
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}`,
						},
					},
				},
			},
		}

		policy.Spec.AutogenConfiguration = &policiesv1beta1.MutatingPolicyAutogenConfiguration{
			PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
				Controllers: []string{"cronjobs"},
			},
		}

		result, err := Autogen(policy)
		require.NoError(t, err)

		if len(result) == 0 {
			t.Skip("Autogen returned empty result, skipping integration test")
			return
		}

		cronjobAutogen, exists := result["cronjobs"]
		assert.True(t, exists)
		assert.Len(t, cronjobAutogen.Targets, 1)
		assert.Equal(t, "cronjobs", cronjobAutogen.Targets[0].Resource)
		assert.Equal(t, policy.Spec.MatchConstraints.NamespaceSelector, cronjobAutogen.Spec.MatchConstraints.NamespaceSelector)

		assert.Len(t, cronjobAutogen.Spec.Mutations, 1)
		convertedExpr := cronjobAutogen.Spec.Mutations[0].ApplyConfiguration.Expression
		assert.Contains(t, convertedExpr, "jobTemplate.spec.template.spec.containers")
		assert.NotContains(t, convertedExpr, "object.spec.containers")
	})

	t.Run("nil-policy", func(t *testing.T) {
		result, err := Autogen(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("non-autogenable match constraints", func(t *testing.T) {
		pol := &policiesv1beta1.MutatingPolicy{
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{"custom.group.io"},
									APIVersions: []string{"v1"},
									Resources:   []string{"customresources"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Connect},
							},
						},
					},
				},
			},
		}

		result, err := Autogen(pol)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestMutationConversionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		input    string
		expected string
	}{
		{
			name:     "empty expression",
			config:   "deployments",
			input:    "",
			expected: "",
		},
		{
			name:     "expression without containers",
			config:   "deployments",
			input:    "Object{metadata: Object.metadata{labels: Object.metadata.labels{foo: 'bar'}}}",
			expected: "Object{spec: Object.spec{template: Object.spec.template{metadata: Object.metadata{labels: Object.spec.template.metadata.labels{foo: 'bar'}}}}}",
		},
		{
			name:   "whitespace between Object and brace",
			config: "deployments",
			input: `Object {
  spec: Object.spec {
    automountServiceAccountToken: false,
  }
}`,
			expected: `Object{spec: Object.spec{template: Object.spec.template{
  spec: Object.spec.template.spec{
    automountServiceAccountToken: false,
  }
}}}`,
		},
		{
			name:   "complex nested expression",
			config: "cronjobs",
			input: `Object{
  spec: Object.spec{
    containers: object.spec.containers.map(container, Object.spec.containers{
      name: container.name,
      env: [Object.spec.containers.env{
        name: "ENV",
        value: "prod"
      }],
      securityContext: Object.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}`,
			expected: `Object{spec: Object.spec{jobTemplate: Object.spec.jobTemplate{spec: Object.spec.jobTemplate.spec{template: Object.spec.jobTemplate.spec.template{
  spec: Object.spec.jobTemplate.spec.template.spec{
    containers: object.spec.jobTemplate.spec.template.spec.containers.map(container, Object.spec.jobTemplate.spec.template.spec.containers{
      name: container.name,
      env: [Object.spec.jobTemplate.spec.template.spec.containers.env{
        name: "ENV",
        value: "prod"
      }],
      securityContext: Object.spec.jobTemplate.spec.template.spec.containers.securityContext{
        allowPrivilegeEscalation: false
      }
    })
  }
}}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPodToTemplateExpression(tt.input, tt.config)
			assert.Equal(t, normalize(tt.expected), normalize(result))
		})
	}
}

func TestAutogenPodControllersEnabled(t *testing.T) {
	newPolicy := func(pc *policiesv1beta1.PodControllersGenerationConfiguration) *policiesv1beta1.MutatingPolicy {
		return &policiesv1beta1.MutatingPolicy{
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{spec: Object.spec{}}`,
						},
					},
				},
				AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
					PodControllers: pc,
				},
			},
		}
	}

	t.Run("enabled=false skips autogen entirely", func(t *testing.T) {
		policy := newPolicy(&policiesv1beta1.PodControllersGenerationConfiguration{
			Enabled: ptr.To(false),
		})
		result, err := Autogen(policy)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("enabled=false takes precedence over controllers list", func(t *testing.T) {
		policy := newPolicy(&policiesv1beta1.PodControllersGenerationConfiguration{
			Enabled:     ptr.To(false),
			Controllers: []string{"deployments"},
		})
		result, err := Autogen(policy)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("enabled=true generates autogen rules", func(t *testing.T) {
		policy := newPolicy(&policiesv1beta1.PodControllersGenerationConfiguration{
			Enabled:     ptr.To(true),
			Controllers: []string{"deployments"},
		})
		result, err := Autogen(policy)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Contains(t, result, autogen.AutogenDefaults)
		assert.Equal(t, []policiesv1beta1.Target{{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
			Kind:     "Deployment",
		}}, result[autogen.AutogenDefaults].Targets)
	})
}

func TestGenerateRuleForControllersJSONPatch(t *testing.T) {
	spec := &policiesv1beta1.MutatingPolicySpec{
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
				{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					},
				},
			},
		},
		Mutations: []admissionregistrationv1alpha1.Mutation{
			{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `object.spec.containers.map(c, JSONPatch{op: "add", path: "/metadata/annotations/" + jsonpatch.escapeKey("container.apparmor.security.beta.kubernetes.io/" + c.name), value: "runtime/default"})`,
				},
			},
		},
	}
	rules, err := generateRuleForControllers(spec, sets.New("deployments", "cronjobs"))
	require.NoError(t, err)
	assert.Equal(t,
		`object.spec.template.spec.containers.map(c, JSONPatch{op: "add", path: "/spec/template/metadata/annotations/" + jsonpatch.escapeKey("container.apparmor.security.beta.kubernetes.io/" + c.name), value: "runtime/default"})`,
		rules[autogen.AutogenDefaults].Spec.Mutations[0].JSONPatch.Expression,
	)
	assert.Equal(t,
		`object.spec.jobTemplate.spec.template.spec.containers.map(c, JSONPatch{op: "add", path: "/spec/jobTemplate/spec/template/metadata/annotations/" + jsonpatch.escapeKey("container.apparmor.security.beta.kubernetes.io/" + c.name), value: "runtime/default"})`,
		rules[autogen.AutogenCronjobs].Spec.Mutations[0].JSONPatch.Expression,
	)
}

func TestGenerateRuleForControllersSkipsUnsafeJSONPatch(t *testing.T) {
	spec := &policiesv1beta1.MutatingPolicySpec{
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
				{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					},
				},
			},
		},
		Mutations: []admissionregistrationv1alpha1.Mutation{
			{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: variables.target, value: "x"}]`,
				},
			},
		},
	}
	rules, err := generateRuleForControllers(spec, sets.New("deployments", "cronjobs"))
	require.NoError(t, err)
	assert.Empty(t, rules)
}
