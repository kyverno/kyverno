package webhook

import (
	"testing"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBuildWebhookRules(t *testing.T) {
	tests := []struct {
		name             string
		vpols            []*kyvernov2alpha1.ValidatingPolicy
		expectedWebhooks []admissionregistrationv1.ValidatingWebhook
	}{
		{
			name: "Single Ignore Policy",
			vpols: []*kyvernov2alpha1.ValidatingPolicy{
				{
					Spec: kyvernov2alpha1.ValidatingPolicySpec{
						ValidatingAdmissionPolicySpec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
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
			vpols: []*kyvernov2alpha1.ValidatingPolicy{
				{
					Spec: kyvernov2alpha1.ValidatingPolicySpec{
						ValidatingAdmissionPolicySpec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
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
			vpols: []*kyvernov2alpha1.ValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-fine-grained-ignore",
					},
					Spec: kyvernov2alpha1.ValidatingPolicySpec{
						WebhookConfiguration: &kyvernov2alpha1.WebhookConfiguration{
							TimeoutSeconds: ptr.To(int32(30)),
						},
						ValidatingAdmissionPolicySpec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
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
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ValidatingPolicyWebhookName + "-ignore-finegrained-test-fine-grained-ignore",
					ClientConfig: newClientConfig("", 0, nil, "/ignore"+config.FineGrainedWebhookPath+"/test-fine-grained-ignore"),
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
			vpols: []*kyvernov2alpha1.ValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-fine-grained-fail",
					},
					Spec: kyvernov2alpha1.ValidatingPolicySpec{
						WebhookConfiguration: &kyvernov2alpha1.WebhookConfiguration{
							TimeoutSeconds: ptr.To(int32(20)),
						},
						ValidatingAdmissionPolicySpec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
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
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ValidatingPolicyWebhookName + "-fail-finegrained-test-fine-grained-fail",
					ClientConfig: newClientConfig("", 0, nil, "/fail"+config.FineGrainedWebhookPath+"/test-fine-grained-fail"),
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
			webhooks := buildWebhookRules("", 0, nil, tt.vpols)
			assert.Equal(t, len(tt.expectedWebhooks), len(webhooks))
			for i, expect := range tt.expectedWebhooks {
				assert.Equal(t, webhooks[i].Name, expect.Name)
				assert.Equal(t, webhooks[i].FailurePolicy, expect.FailurePolicy)
				assert.Equal(t, len(webhooks[i].Rules), len(expect.Rules))

				if expect.MatchConditions != nil {
					assert.Equal(t, webhooks[i].MatchConditions, expect.MatchConditions)
				}
				if expect.MatchPolicy != nil {
					assert.Equal(t, webhooks[i].MatchPolicy, expect.MatchPolicy)
				}
				if expect.TimeoutSeconds != nil {
					assert.Equal(t, webhooks[i].TimeoutSeconds, expect.TimeoutSeconds)
				}
				if expect.ClientConfig.Service != nil {
					assert.Equal(t, webhooks[i].ClientConfig.Service.Path, expect.ClientConfig.Service.Path)
				}
			}
		})
	}
}
