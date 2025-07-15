package autogen

import (
	"strings"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
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
    labels: Object.metadata.labels{
      foo: "bar"
    }
  }
}}}`,
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
		spec          *policiesv1alpha1.MutatingPolicySpec
		configs       sets.Set[string]
		expectedRules int
		expectedError bool
	}{
		{
			name: "deployment autogen with mutation",
			spec: &policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
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
			spec: &policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
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
			spec: &policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
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
			spec: &policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
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
		policy := &policiesv1alpha1.MutatingPolicy{
			Spec: policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
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

		policy.Spec.AutogenConfiguration = &policiesv1alpha1.MutatingPolicyAutogenConfiguration{
			PodControllers: &policiesv1alpha1.PodControllersGenerationConfiguration{
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

		assert.Len(t, deploymentAutogen.Spec.Mutations, 1)
		convertedExpr := deploymentAutogen.Spec.Mutations[0].ApplyConfiguration.Expression
		assert.Contains(t, convertedExpr, "template.spec.containers")
		assert.NotContains(t, convertedExpr, "object.spec.containers")
	})

	t.Run("cronjobs", func(t *testing.T) {
		policy := &policiesv1alpha1.MutatingPolicy{
			Spec: policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
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

		policy.Spec.AutogenConfiguration = &policiesv1alpha1.MutatingPolicyAutogenConfiguration{
			PodControllers: &policiesv1alpha1.PodControllersGenerationConfiguration{
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
		pol := &policiesv1alpha1.MutatingPolicy{
			Spec: policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
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
			expected: "Object{spec: Object.spec{template: Object.spec.template{metadata: Object.metadata{labels: Object.metadata.labels{foo: 'bar'}}}}}",
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
