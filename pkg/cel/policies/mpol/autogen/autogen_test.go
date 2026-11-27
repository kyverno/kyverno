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
  metadata: Object.spec.template.metadata{
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
			result, _ := convertPodToTemplateExpression(tt.input, tt.config)
			assert.Equal(t, normalize(tt.expected), normalize(result))
		})
	}
}

// TestConvertPodToTemplateExpressionSupportedShapes covers which expression
// shapes the conversion claims to handle. A shape it cannot rewrite reports
// false, so its target stays in extraction mode instead of being wrapped by
// guesswork.
func TestConvertPodToTemplateExpressionSupportedShapes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantWrapped bool
	}{
		{name: "object constructor", input: `Object{spec: Object.spec{}}`, wantWrapped: true},
		{name: "object constructor with trailing comment", input: "Object{spec: Object.spec{}} // note", wantWrapped: true},
		{name: "conditional of constructors", input: `true ? Object{metadata: Object.metadata{}} : Object{}`, wantWrapped: true},
		{
			name:        "triple-quoted literal with an apostrophe in the condition",
			input:       `'''it's''' == "it's" ? Object{spec: Object.spec{hostNetwork: false}} : Object{}`,
			wantWrapped: true,
		},
		{
			name:        "literal containing conditional punctuation",
			input:       `"a ? b : c" == "a ? b : c" ? Object{spec: Object.spec{}} : Object{spec: Object.spec{}}`,
			wantWrapped: true,
		},
		{
			name:        "raw triple-quoted literal containing quotes",
			input:       "r'''a \" b''' == \"x\" ? Object{spec: Object.spec{}} : Object{spec: Object.spec{}}",
			wantWrapped: true,
		},
		{name: "nested conditional of constructors", input: `a ? Object{} : b ? Object{} : Object{}`, wantWrapped: true},
		{name: "condition whose branch is not a constructor", input: `a ? variables.patch : Object{}`, wantWrapped: false},
		{name: "variable reference", input: `variables.patch`, wantWrapped: false},
		{name: "selector expression", input: `object.spec.containers`, wantWrapped: false},
		{name: "method call on a constructor", input: `Object{spec: Object.spec{}}.merge(other)`, wantWrapped: false},
		{name: "unclosed constructor", input: `Object{custom: "val"`, wantWrapped: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, wrapped := convertPodToTemplateExpression(tt.input, "deployments")
			assert.Equal(t, tt.wantWrapped, wrapped)
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
		name       string
		config     string
		input      string
		expected   string
		expectedOk bool
	}{
		{name: "empty expression", config: "deployments", input: "", expected: "", expectedOk: false},
		{name: "expression without containers", config: "deployments", input: "Object{metadata: Object.metadata{labels: Object.metadata.labels{foo: 'bar'}}}", expected: "Object{spec: Object.spec{template: Object.spec.template{metadata: Object.spec.template.metadata{labels: Object.spec.template.metadata.labels{foo: 'bar'}}}}}", expectedOk: true},
		{name: "whitespace between Object and brace", config: "deployments", input: "Object {\n  spec: Object.spec {\n    automountServiceAccountToken: false,\n  }\n}", expected: "Object{spec: Object.spec{template: Object.spec.template{\n  spec: Object.spec.template.spec{\n    automountServiceAccountToken: false,\n  }\n}}}", expectedOk: true},
		{name: "complex nested expression", config: "cronjobs", input: "Object{\n  spec: Object.spec{\n    containers: object.spec.containers.map(container, Object.spec.containers{\n      name: container.name,\n      env: [Object.spec.containers.env{\n        name: \"ENV\",\n        value: \"prod\"\n      }],\n      securityContext: Object.spec.containers.securityContext{\n        allowPrivilegeEscalation: false\n      }\n    })\n  }\n}", expected: "Object{spec: Object.spec{jobTemplate: Object.spec.jobTemplate{spec: Object.spec.jobTemplate.spec{template: Object.spec.jobTemplate.spec.template{\n  spec: Object.spec.jobTemplate.spec.template.spec{\n    containers: object.spec.jobTemplate.spec.template.spec.containers.map(container, Object.spec.jobTemplate.spec.template.spec.containers{\n      name: container.name,\n      env: [Object.spec.jobTemplate.spec.template.spec.containers.env{\n        name: \"ENV\",\n        value: \"prod\"\n      }],\n      securityContext: Object.spec.jobTemplate.spec.template.spec.containers.securityContext{\n        allowPrivilegeEscalation: false\n      }\n    })\n  }\n}}}}}", expectedOk: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, ok := convertPodToTemplateExpression(tt.input, tt.config)
			assert.Equal(t, normalize(tt.expected), normalize(out))
			assert.Equal(t, tt.expectedOk, ok, "unexpected conversion flag for %s", tt.name)
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

// TestAutogenExplicitBuiltInControllersUseConfigsMap checks that naming a
// built-in controller by its explicit canonical <resource>.<version>.<group>
// form generates the same rule as the bare built-in name. The explicit form
// used to resolve through the extraction-mode path, and the MutatingPolicy
// engine skips every extraction-mode mutation, so the policy silently did not
// mutate the built-in workload at all.
func TestAutogenExplicitBuiltInControllersUseConfigsMap(t *testing.T) {
	t.Cleanup(func() {
		autogen.BareNameResolver = nil
		autogen.KindResolver = nil
	})
	autogen.BareNameResolver = nil
	autogen.KindResolver = nil

	tests := []struct {
		name          string
		controller    string
		wantRef       string
		wantTarget    policiesv1beta1.Target
		wantExprHas   []string
		wantExprLacks []string
	}{
		{
			name:       "deployments explicit form resolves to the built-in config",
			controller: "deployments.v1.apps",
			wantRef:    autogen.AutogenDefaults,
			wantTarget: policiesv1beta1.Target{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
				Kind:     "Deployment",
			},
			wantExprHas: []string{
				"object.spec.template.spec.containers",
				"Object.spec.template.spec.containers{",
				"Object{spec: Object.spec{template: Object.spec.template{",
				"spec: Object.spec.template.spec{",
			},
			wantExprLacks: []string{
				"object.spec.containers",
				"jobTemplate",
			},
		},
		{
			name:       "jobs explicit form resolves to the built-in config",
			controller: "jobs.v1.batch",
			wantRef:    autogen.AutogenDefaults,
			wantTarget: policiesv1beta1.Target{
				Group:    "batch",
				Version:  "v1",
				Resource: "jobs",
				Kind:     "Job",
			},
			wantExprHas: []string{
				"object.spec.template.spec.containers",
				"Object{spec: Object.spec{template: Object.spec.template{",
				"spec: Object.spec.template.spec{",
			},
			wantExprLacks: []string{
				"object.spec.containers",
				"jobTemplate",
			},
		},
		{
			// A CronJob's pod template sits one level deeper than the other
			// built-in workloads, so its explicit canonical form must resolve
			// to the cronjobs config, not the defaults one: the mutation is
			// rewritten to the jobTemplate path and wrapped in the matching
			// constructors.
			name:       "cronjobs explicit form resolves to the cronjob config",
			controller: "cronjobs.v1.batch",
			wantRef:    autogen.AutogenCronjobs,
			wantTarget: policiesv1beta1.Target{
				Group:    "batch",
				Version:  "v1",
				Resource: "cronjobs",
				Kind:     "CronJob",
			},
			wantExprHas: []string{
				"object.spec.jobTemplate.spec.template.spec.containers",
				"Object.spec.jobTemplate.spec.template.spec.containers{",
				"Object{spec: Object.spec{jobTemplate: Object.spec.jobTemplate{spec: Object.spec.jobTemplate.spec{template: Object.spec.jobTemplate.spec.template{",
			},
			wantExprLacks: []string{
				"object.spec.containers",
				"Object{spec: Object.spec{template: Object.spec.template{",
			},
		},
		{
			// Custom workload CRDs keep using their own extraction-mode
			// path; this change must not affect them.
			name:       "custom explicit form still generates its own extraction target",
			controller: "jobsets.v1alpha2.jobset.x-k8s.io",
			wantRef:    autogen.ExtractionReplacementsRef,
			wantTarget: policiesv1beta1.Target{
				Group:    "jobset.x-k8s.io",
				Version:  "v1alpha2",
				Resource: "jobsets",
				Kind:     "Jobset",
			},
			wantExprHas: []string{
				"object.spec.containers",
			},
			wantExprLacks: []string{
				"object.spec.template.spec",
				"jobTemplate",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.MutatingPolicy{
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
					AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
						PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
							Controllers: []string{tt.controller},
						},
					},
				},
			}

			result, err := Autogen(policy)
			require.NoError(t, err)

			generated, ok := result[tt.wantRef]
			require.True(t, ok, "expected a %q rule, got refs %v", tt.wantRef, sets.List(sets.KeySet(result)))
			assert.Equal(t, []policiesv1beta1.Target{tt.wantTarget}, generated.Targets)

			require.Len(t, generated.Spec.Mutations, 1)
			expr := generated.Spec.Mutations[0].ApplyConfiguration.Expression
			// A built-in target's pod template lives at a fixed path - deeper
			// for CronJobs than for the other controllers - so the mutation is
			// rewritten to that path and wrapped in its constructors. An
			// extraction-mode target has no fixed path and is left untouched.
			for _, want := range tt.wantExprHas {
				assert.Contains(t, expr, want)
			}
			for _, unwanted := range tt.wantExprLacks {
				assert.NotContains(t, expr, unwanted)
			}
		})
	}
}

func TestRewriteExceptions(t *testing.T) {
	polex := &policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "match",
					Expression: "object.spec.containers.exists(c, c.name == 'nginx')",
				},
			},
		},
	}
	res, err := RewriteExceptions([]*policiesv1beta1.PolicyException{polex}, autogen.AutogenDefaults)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "object.spec.template.spec.containers.exists(c, c.name == 'nginx')", res[0].Spec.MatchConditions[0].Expression)
}

func newMutatingPolicyForController(controllers []string, mutations []admissionregistrationv1alpha1.Mutation) *policiesv1beta1.MutatingPolicy {
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
			Mutations: mutations,
			AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: controllers,
				},
			},
		},
	}
}

// TestAutogenConditionalMutationWrappedForPodTemplate covers a mutation whose
// outer syntax is a conditional instead of a bare Object{...} constructor. Every
// branch is pod-shaped, so each one must be wrapped in the workload's template
// constructors: otherwise the metadata mutation labels the workload itself
// rather than its pod template.
func TestAutogenConditionalMutationWrappedForPodTemplate(t *testing.T) {
	policy := newMutatingPolicyForController(
		[]string{"deployments.v1.apps"},
		[]admissionregistrationv1alpha1.Mutation{{
			ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
				Expression: `object.metadata.labels['env'] == 'prod' ? Object{metadata: Object.metadata{labels: {'team': 'dev'}}} : Object{}`,
			},
		}},
	)

	result, err := Autogen(policy)
	require.NoError(t, err)
	generated, ok := result[autogen.AutogenDefaults]
	require.True(t, ok, "expected a %q rule, got refs %v", autogen.AutogenDefaults, sets.List(sets.KeySet(result)))
	require.Len(t, generated.Spec.Mutations, 1)

	expr := generated.Spec.Mutations[0].ApplyConfiguration.Expression
	assert.Equal(t, normalize(`object.spec.template.metadata.labels['env'] == 'prod' ?
Object{spec: Object.spec{template: Object.spec.template{metadata: Object.spec.template.metadata{labels: {'team': 'dev'}}}}} :
Object{spec: Object.spec{template: Object.spec.template{}}}`), normalize(expr))
}

// TestAutogenMetadataConstructorQualifiedForPodTemplate covers the constructor
// type names inside a wrapped mutation. The type-path check in the ApplyConfiguration
// compiler requires every nested ObjectVal's type name to match its field path from
// the root, so a metadata constructor nested under spec.template must be named
// Object.spec.template.metadata (deeper for a CronJob), not Object.metadata.
func TestAutogenMetadataConstructorQualifiedForPodTemplate(t *testing.T) {
	tests := []struct {
		name       string
		controller string
		input      string
		wantRef    string
		wantExpr   string
	}{
		{
			name:       "deployments",
			controller: "deployments.v1.apps",
			wantRef:    autogen.AutogenDefaults,
			wantExpr: `Object{spec: Object.spec{template: Object.spec.template{
  metadata: Object.spec.template.metadata{labels: {'team': 'dev'}}
}}}`,
		},
		{
			name:       "cronjobs",
			controller: "cronjobs.v1.batch",
			wantRef:    autogen.AutogenCronjobs,
			wantExpr: `Object{spec: Object.spec{jobTemplate: Object.spec.jobTemplate{spec: Object.spec.jobTemplate.spec{template: Object.spec.jobTemplate.spec.template{
  metadata: Object.spec.jobTemplate.spec.template.metadata{labels: {'team': 'dev'}}
}}}}}`,
		},
		{
			// CEL permits whitespace around the dot, so the type name must be
			// qualified through the selector scanner, not a literal prefix match.
			name:       "deployments spaced constructor",
			controller: "deployments.v1.apps",
			input:      `Object{metadata: Object . metadata {labels: {'team': 'dev'}}}`,
			wantRef:    autogen.AutogenDefaults,
			wantExpr: `Object{spec: Object.spec{template: Object.spec.template{
  metadata: Object.spec.template.metadata {labels: {'team': 'dev'}}
}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			if input == "" {
				input = `Object{metadata: Object.metadata{labels: {'team': 'dev'}}}`
			}
			policy := newMutatingPolicyForController(
				[]string{tt.controller},
				[]admissionregistrationv1alpha1.Mutation{{
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: input,
					},
				}},
			)

			result, err := Autogen(policy)
			require.NoError(t, err)
			generated, ok := result[tt.wantRef]
			require.True(t, ok, "expected a %q rule, got refs %v", tt.wantRef, sets.List(sets.KeySet(result)))
			require.Len(t, generated.Spec.Mutations, 1)

			expr := generated.Spec.Mutations[0].ApplyConfiguration.Expression
			assert.Equal(t, normalize(tt.wantExpr), normalize(expr))
		})
	}
}

// TestAutogenVariableConstructorQualifiedForPodTemplate covers a pod object
// constructor supplied through a variable: the mutation places it under
// spec.template.metadata, so the variable's own constructor type name has to be
// template-qualified too, or the generated patch is rejected with a type
// mismatch.
func TestAutogenVariableConstructorQualifiedForPodTemplate(t *testing.T) {
	policy := newMutatingPolicyForController(
		[]string{"deployments.v1.apps"},
		[]admissionregistrationv1alpha1.Mutation{{
			ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
				Expression: `Object{metadata: variables.podMetadata}`,
			},
		}},
	)
	policy.Spec.Variables = []admissionregistrationv1.Variable{{
		Name:       "podMetadata",
		Expression: `Object.metadata{labels: {'team': 'dev'}}`,
	}}

	result, err := Autogen(policy)
	require.NoError(t, err)
	generated, ok := result[autogen.AutogenDefaults]
	require.True(t, ok, "expected a %q rule, got refs %v", autogen.AutogenDefaults, sets.List(sets.KeySet(result)))

	require.Len(t, generated.Spec.Variables, 1)
	assert.Equal(t, normalize(`Object.spec.template.metadata{labels: {'team': 'dev'}}`), normalize(generated.Spec.Variables[0].Expression))
	require.Len(t, generated.Spec.Mutations, 1)
	assert.Equal(t, normalize(`Object{spec: Object.spec{template: Object.spec.template{metadata: variables.podMetadata}}}`), normalize(generated.Spec.Mutations[0].ApplyConfiguration.Expression))
}

// TestAutogenWildcardBuiltInVersionKeepsWildcardTarget covers an explicit
// built-in controller with the "*" API version. The target must keep "*" so the
// generated resource rule matches every version, and the built-in replacement
// configuration must still rewrite the mutation to the pod template.
func TestAutogenWildcardBuiltInVersionKeepsWildcardTarget(t *testing.T) {
	policy := newMutatingPolicyForController(
		[]string{"deployments.*.apps"},
		[]admissionregistrationv1alpha1.Mutation{{
			ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
				Expression: `Object{spec: Object.spec{containers: []}}`,
			},
		}},
	)

	result, err := Autogen(policy)
	require.NoError(t, err)
	generated, ok := result[autogen.AutogenDefaults]
	require.True(t, ok, "expected a %q rule, got refs %v", autogen.AutogenDefaults, sets.List(sets.KeySet(result)))
	assert.Equal(t, []policiesv1beta1.Target{{
		Group:    "apps",
		Version:  "*",
		Resource: "deployments",
		Kind:     "Deployment",
	}}, generated.Targets)
	require.Len(t, generated.Spec.Mutations, 1)
	assert.Contains(t, generated.Spec.Mutations[0].ApplyConfiguration.Expression, "Object.spec.template.spec")
}

// TestAutogenJSONPatchExplicitBuiltInStaysInExtractionMode covers JSONPatch
// mutations on a controller named by its explicit canonical built-in form.
// Converting a Pod patch to the workload's template is not implemented, and
// applying it to the workload itself would mutate the wrong object (a Pod patch
// to /metadata/labels/checked labels the Deployment), so that target stays in
// extraction mode, where the MutatingPolicy engine skips it.
func TestAutogenJSONPatchExplicitBuiltInStaysInExtractionMode(t *testing.T) {
	expression := `[JSONPatch{op: "add", path: "/metadata/labels/checked", value: "true"}]`
	policy := newMutatingPolicyForController(
		[]string{"deployments.v1.apps"},
		[]admissionregistrationv1alpha1.Mutation{{
			JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: expression},
		}},
	)

	result, err := Autogen(policy)
	require.NoError(t, err)
	require.Contains(t, result, autogen.ExtractionReplacementsRef, "got refs %v", sets.List(sets.KeySet(result)))
	assert.NotContains(t, result, autogen.AutogenDefaults)

	generated := result[autogen.ExtractionReplacementsRef]
	require.Len(t, generated.Spec.Mutations, 1)
	require.NotNil(t, generated.Spec.Mutations[0].JSONPatch)
	assert.Equal(t, expression, generated.Spec.Mutations[0].JSONPatch.Expression)
}

// TestAutogenUnconvertibleMutationStaysInExtractionMode covers a mutation whose
// expression cannot be rewritten to the pod template (here a variable reference
// rather than an Object{...} constructor). Guessing a conversion would apply a
// Pod-shaped mutation to the workload itself, so the target stays in extraction
// mode instead.
func TestAutogenUnconvertibleMutationStaysInExtractionMode(t *testing.T) {
	t.Run("convertible mutation on bare built-in stays in defaults", func(t *testing.T) {
		policy := newMutatingPolicyForController([]string{"deployments"}, []admissionregistrationv1alpha1.Mutation{{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `Object{spec: Object.spec{containers: []}}`}}})
		inputCopy := policy.DeepCopy()
		result, err := Autogen(policy)
		require.NoError(t, err)
		assert.Contains(t, result, autogen.AutogenDefaults, "got refs %v", sets.List(sets.KeySet(result)))
		assert.NotContains(t, result, ExtractionReplacementsRef)
		assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
	})
	t.Run("unconvertible mutation on bare built-in falls back to extraction", func(t *testing.T) {
		policy := newMutatingPolicyForController([]string{"deployments"}, []admissionregistrationv1alpha1.Mutation{{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}}})
		inputCopy := policy.DeepCopy()
		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Contains(t, result, ExtractionReplacementsRef, "got refs %v", sets.List(sets.KeySet(result)))
		assert.NotContains(t, result, autogen.AutogenDefaults)
		generated := result[ExtractionReplacementsRef]
		require.Len(t, generated.Spec.Mutations, 1)
		assert.Equal(t, `variables.podPatch`, generated.Spec.Mutations[0].ApplyConfiguration.Expression)
		assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
	})
	t.Run("unconvertible mutation on remapped explicit form falls back to extraction", func(t *testing.T) {
		policy := newMutatingPolicyForController([]string{"deployments.v1.apps"}, []admissionregistrationv1alpha1.Mutation{{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}}})
		inputCopy := policy.DeepCopy()
		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Contains(t, result, ExtractionReplacementsRef, "got refs %v", sets.List(sets.KeySet(result)))
		assert.NotContains(t, result, autogen.AutogenDefaults)
		generated := result[ExtractionReplacementsRef]
		require.Len(t, generated.Spec.Mutations, 1)
		assert.Equal(t, `variables.podPatch`, generated.Spec.Mutations[0].ApplyConfiguration.Expression)
		assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
	})
	t.Run("supported and unsupported mutations keep all original expressions", func(t *testing.T) {
		policy := newMutatingPolicyForController([]string{"deployments.v1.apps"}, []admissionregistrationv1alpha1.Mutation{{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `Object{spec: Object.spec{containers: []}}`}}, {ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}}})
		policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "podPatch", Expression: `Object{spec: Object.spec{containers: []}}`}}
		inputCopy := policy.DeepCopy()
		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Contains(t, result, ExtractionReplacementsRef, "got refs %v", sets.List(sets.KeySet(result)))
		assert.NotContains(t, result, autogen.AutogenDefaults)
		generated := result[ExtractionReplacementsRef]
		require.Len(t, generated.Spec.Mutations, 2)
		assert.Equal(t, `Object{spec: Object.spec{containers: []}}`, generated.Spec.Mutations[0].ApplyConfiguration.Expression)
		assert.Equal(t, `variables.podPatch`, generated.Spec.Mutations[1].ApplyConfiguration.Expression)
		assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
	})
}

func TestConvertPodToTemplateExpression_StringLiteralsWithBraces(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		config     string
		expected   string
		expectedOk bool
	}{
		{name: "string with only closing brace", input: `Object{spec: Object.spec{msg: "foo } bar"}}`, config: "deployments", expected: `Object{spec: Object.spec{template: Object.spec.template{spec: Object.spec.template.spec{msg: "foo } bar"}}}}`, expectedOk: true},
		{name: "string with only opening brace", input: `Object{spec: Object.spec{msg: "foo { bar"}}`, config: "deployments", expected: `Object{spec: Object.spec{template: Object.spec.template{spec: Object.spec.template.spec{msg: "foo { bar"}}}}`, expectedOk: true},
		{name: "backtick raw string with closing brace", input: "Object{spec: Object.spec{cmd: `cmd } arg`}}", config: "deployments", expected: "Object{spec: Object.spec{template: Object.spec.template{spec: Object.spec.template.spec{cmd: `cmd } arg`}}}}", expectedOk: true},
		{name: "unclosed constructor remains unwrapped", input: `Object{custom: "val"`, config: "deployments", expected: `Object{custom: "val"`, expectedOk: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, ok := convertPodToTemplateExpression(tt.input, tt.config)
			assert.Equal(t, tt.expected, out)
			assert.Equal(t, tt.expectedOk, ok, "unexpected conversion flag for %s", tt.name)
		})
	}
}
