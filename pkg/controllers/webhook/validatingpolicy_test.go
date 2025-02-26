package webhook

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBuildWebhookRules(t *testing.T) {
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
					ClientConfig: newClientConfig("", 0, nil, "/vpol/ignore"+config.FineGrainedWebhookPath+"/test-fine-grained-ignore"),
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
					ClientConfig: newClientConfig("", 0, nil, "/vpol/fail"+config.FineGrainedWebhookPath+"/test-fine-grained-fail"),
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
			var vpols []policiesv1alpha1.ValidatingPolicyInterface
			for _, vpol := range tt.vpols {
				vpols = append(vpols, vpol)
			}
			webhooks := buildWebhookRules(config.NewDefaultConfiguration(false), "", 0, nil, vpols)
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
