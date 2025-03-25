package webhook

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	celautogen "github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBuildWebhookRules_ValidatingPolicy(t *testing.T) {
	tests := []struct {
		name             string
		vpols            []*policiesv1alpha1.ValidatingPolicy
		expectedWebhooks []admissionregistrationv1.ValidatingWebhook
	}{
		{
			name: "Single Ignore Policy",
			vpols: []*policiesv1alpha1.ValidatingPolicy{
				{
					Spec: policiesv1alpha1.ValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.ValidatingPolicyWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
		{
			name: "Single Fail Policy",
			vpols: []*policiesv1alpha1.ValidatingPolicy{
				{
					Spec: policiesv1alpha1.ValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Fail),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.ValidatingPolicyWebhookName + "-fail",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy: ptr.To(admissionregistrationv1.Fail),
				},
			},
		},
		{
			name: "Fine-Grained Ignore Policy",
			vpols: []*policiesv1alpha1.ValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-fine-grained-ignore",
					},
					Spec: policiesv1alpha1.ValidatingPolicySpec{
						WebhookConfiguration: &policiesv1alpha1.WebhookConfiguration{
							TimeoutSeconds: ptr.To(int32(30)),
						},
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							MatchPolicy: ptr.To(admissionregistrationv1.Exact),
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ValidatingPolicyWebhookName + "-ignore-finegrained-test-fine-grained-ignore",
					ClientConfig: newClientConfig("", 0, nil, "/policies/vpol/validate/ignore"+config.FineGrainedWebhookPath+"/test-fine-grained-ignore"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy:  ptr.To(admissionregistrationv1.Ignore),
					TimeoutSeconds: ptr.To(int32(30)),
					MatchPolicy:    ptr.To(admissionregistrationv1.Exact),
				},
			},
		},
		{
			name: "Fine-Grained Fail Policy",
			vpols: []*policiesv1alpha1.ValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-fine-grained-fail",
					},
					Spec: policiesv1alpha1.ValidatingPolicySpec{
						WebhookConfiguration: &policiesv1alpha1.WebhookConfiguration{
							TimeoutSeconds: ptr.To(int32(20)),
						},
						FailurePolicy: ptr.To(admissionregistrationv1.Fail),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							MatchPolicy: ptr.To(admissionregistrationv1.Exact),
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "exclude-leases",
								Expression: "!(request.resource.group == 'coordination.k8s.io' && request.resource.resource == 'leases')",
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ValidatingPolicyWebhookName + "-fail-finegrained-test-fine-grained-fail",
					ClientConfig: newClientConfig("", 0, nil, "/policies/vpol/validate/fail"+config.FineGrainedWebhookPath+"/test-fine-grained-fail"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy:  ptr.To(admissionregistrationv1.Fail),
					TimeoutSeconds: ptr.To(int32(20)),
					MatchPolicy:    ptr.To(admissionregistrationv1.Exact),
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "exclude-leases",
							Expression: "!(request.resource.group == 'coordination.k8s.io' && request.resource.resource == 'leases')",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var vpols []engineapi.GenericPolicy
			for _, vpol := range tt.vpols {
				vpols = append(vpols, engineapi.NewValidatingPolicy(vpol))
			}
			webhooks := buildWebhookRules(config.NewDefaultConfiguration(false), "", config.ValidatingPolicyWebhookName,
				config.PolicyServicePath+config.ValidatingPolicyServicePath+config.ValidatingWebhookServicePath, 0, nil, vpols)
			assert.Equal(t, len(tt.expectedWebhooks), len(webhooks))
			for i, expect := range tt.expectedWebhooks {
				assert.Equal(t, expect.Name, webhooks[i].Name)
				assert.Equal(t, expect.FailurePolicy, webhooks[i].FailurePolicy)
				assert.Equal(t, len(expect.Rules), len(webhooks[i].Rules))

				if expect.MatchConditions != nil {
					assert.Equal(t, expect.MatchConditions, webhooks[i].MatchConditions)
				}
				if expect.MatchPolicy != nil {
					assert.Equal(t, expect.MatchPolicy, webhooks[i].MatchPolicy)
				}
				if expect.TimeoutSeconds != nil {
					assert.Equal(t, expect.TimeoutSeconds, webhooks[i].TimeoutSeconds)
				}
				if expect.ClientConfig.Service != nil {
					assert.Equal(t, *webhooks[i].ClientConfig.Service.Path, *expect.ClientConfig.Service.Path)
				}
			}
		})
	}
}

func TestBuildWebhookRules_ImageVerificationPolicy(t *testing.T) {
	tests := []struct {
		name             string
		ivpols           []*policiesv1alpha1.ImageValidatingPolicy
		expectedWebhooks []admissionregistrationv1.ValidatingWebhook
	}{
		{
			name: "Autogen Single Ignore Policy",
			ivpols: []*policiesv1alpha1.ImageValidatingPolicy{
				{
					Spec: policiesv1alpha1.ImageValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.ImageVerificationPolicyValidateWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"jobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"cronjobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
		{
			name: "Autogen Fine-grained Ignore Policy",
			ivpols: []*policiesv1alpha1.ImageValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ivpol-sample",
						Annotations: map[string]string{
							kyverno.AnnotationAutogenControllers: "deployments,jobs,cronjobs",
						},
					},
					Spec: policiesv1alpha1.ImageValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "check-prod-label",
								Expression: "has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'",
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ImageVerificationPolicyValidateWebhookName + "-ignore-finegrained-ivpol-sample",
					ClientConfig: newClientConfig("", 0, nil, "/policies/ivpol/validate/ignore"+config.FineGrainedWebhookPath+"/ivpol-sample"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"jobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"cronjobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "check-prod-label",
							Expression: "!(object.kind == 'Pod') || has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'",
						},
						{
							Name:       "autogen-check-prod-label",
							Expression: celautogen.PodControllersMatchConditionExpression + "has(object.spec.template.metadata.labels) && has(object.spec.template.metadata.labels.prod) && object.spec.template.metadata.labels.prod == 'true'",
						},
						{
							Name:       "autogen-cronjobs-check-prod-label",
							Expression: celautogen.CronJobMatchConditionExpression + "has(object.spec.jobTemplate.spec.template.metadata.labels) && has(object.spec.jobTemplate.spec.template.metadata.labels.prod) && object.spec.jobTemplate.spec.template.metadata.labels.prod == 'true'",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ivpols []engineapi.GenericPolicy
			for _, ivpol := range tt.ivpols {
				ivpols = append(ivpols, engineapi.NewImageVerificationPolicy(ivpol))
			}
			webhooks := buildWebhookRules(config.NewDefaultConfiguration(false), "", config.ImageVerificationPolicyValidateWebhookName,
				config.PolicyServicePath+config.ImageVerificationPolicyServicePath+config.ValidatingWebhookServicePath, 0, nil, ivpols)
			assert.Equal(t, len(tt.expectedWebhooks), len(webhooks), tt.name)
			for i, expect := range tt.expectedWebhooks {
				assert.Equal(t, expect.Name, webhooks[i].Name)
				assert.Equal(t, expect.FailurePolicy, webhooks[i].FailurePolicy)
				assert.Equal(t, len(expect.Rules), len(webhooks[i].Rules), fmt.Sprintf("expected: %v,\n got: %v", expect.Rules, webhooks[i].Rules))

				if expect.MatchConditions != nil {
					for m, mExpect := range expect.MatchConditions {
						for n, mActual := range webhooks[i].MatchConditions {
							if mExpect.Name != mActual.Name {
								continue
							}
							assert.Equal(t, expect.MatchConditions[m], webhooks[i].MatchConditions[n])
						}
					}

				}
				if expect.MatchPolicy != nil {
					assert.Equal(t, expect.MatchPolicy, webhooks[i].MatchPolicy)
				}
				if expect.TimeoutSeconds != nil {
					assert.Equal(t, expect.TimeoutSeconds, webhooks[i].TimeoutSeconds)
				}
				if expect.ClientConfig.Service != nil {
					assert.Equal(t, *webhooks[i].ClientConfig.Service.Path, *expect.ClientConfig.Service.Path)
				}
			}
		})
	}
}
